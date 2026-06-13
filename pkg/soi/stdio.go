package soi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Source-of-Intelligence/soi-vos"
	"github.com/tetratelabs/wazero"
	wazeroapi "github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// StdioPlugin executes standard Go WASM modules compiled for wasip1
// via WASI stdin/stdout. The plugin reads JSON input from stdin and
// writes its result to stdout.
//
// Usage:
//
//	plugin, err := soi.NewStdioPlugin(ctx, wasmBytes, host, sandboxDir)
//	result, err := plugin.Execute(ctx, "my_tool", map[string]interface{}{...})
type StdioPlugin struct {
	// wasmBytes are stored so the module can be (re-)compiled if needed.
	wasmBytes []byte

	// runtime is the shared wazero.Runtime for this plugin.
	runtime wazero.Runtime

	// compiled is the pre-compiled module, cached across Execute calls.
	compiled wazero.CompiledModule

	// sandboxDir is the directory mounted at /sandbox for the plugin.
	sandboxDir string
}

// NewStdioPlugin creates a new StdioPlugin and pre-compiles the WASM module.
// The host provides host functions (soi_log, soi_now, etc.) registered into
// the runtime. If sandboxDir is non-empty, it is mounted at /sandbox for the
// plugin's file system access.
func NewStdioPlugin(ctx context.Context, wasmBytes []byte, host vos.HostFunctions, sandboxDir string) (*StdioPlugin, error) {
	if len(wasmBytes) == 0 {
		return nil, fmt.Errorf("empty wasm bytes")
	}

	rt := wazero.NewRuntime(ctx)

	// --- 1) WASI: override proc_exit to prevent runtime self-termination ---
	wasiBuilder := rt.NewHostModuleBuilder(wasi_snapshot_preview1.ModuleName)
	wasi_snapshot_preview1.NewFunctionExporter().ExportFunctions(wasiBuilder)
	wasiBuilder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, exitCode uint32) {
			// no-op: keep runtime alive across multiple instantiations.
		}).
		Export("proc_exit")
	if _, err := wasiBuilder.Instantiate(ctx); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("instantiate WASI: %w", err)
	}

	// --- 2) SOI host functions (soi_log, soi_now, soi_sandbox_*, etc.) ---
	soiBuilder := rt.NewHostModuleBuilder(vos.HostModuleName)

	// soi_log(level:i32, ptr:i64, len:i64)
	soiBuilder.NewFunctionBuilder().
		WithGoModuleFunction(wazeroapi.GoModuleFunc(func(ctx context.Context, mod wazeroapi.Module, stack []uint64) {
			level := int32(stack[0])
			ptr := uint32(stack[1])
			length := uint32(stack[2])
			host.Log(level, readStrFromModule(mod, ptr, length))
		}), []wazeroapi.ValueType{wazeroapi.ValueTypeI32, wazeroapi.ValueTypeI64, wazeroapi.ValueTypeI64}, nil).
		Export(vos.HostLog)

	// soi_now() -> i64
	soiBuilder.NewFunctionBuilder().
		WithGoModuleFunction(wazeroapi.GoModuleFunc(func(ctx context.Context, mod wazeroapi.Module, stack []uint64) {
			stack[0] = uint64(host.Now())
		}), nil, []wazeroapi.ValueType{wazeroapi.ValueTypeI64}).
		Export(vos.HostNow)

	// soi_random(ptr:i64, len:i64)
	soiBuilder.NewFunctionBuilder().
		WithGoModuleFunction(wazeroapi.GoModuleFunc(func(ctx context.Context, mod wazeroapi.Module, stack []uint64) {
			ptr := uint32(stack[0])
			length := uint32(stack[1])
			if length > 0 {
				buf := make([]byte, length)
				_ = host.Random(buf)
				mod.Memory().Write(ptr, buf)
			}
		}), []wazeroapi.ValueType{wazeroapi.ValueTypeI64, wazeroapi.ValueTypeI64}, nil).
		Export(vos.HostRandom)

	// soi_sandbox_read(path_ptr:i64, path_len:i64) -> i64
	soiBuilder.NewFunctionBuilder().
		WithGoModuleFunction(wazeroapi.GoModuleFunc(func(ctx context.Context, mod wazeroapi.Module, stack []uint64) {
			ptr := uint32(stack[0])
			length := uint32(stack[1])
			path := readStrFromModule(mod, ptr, length)
			data, err := host.SandboxRead(path)
			if err != nil {
				data = []byte(`{"error":"` + err.Error() + `"}`)
			}
			stack[0] = writePackedToModule(mod, data)
		}), []wazeroapi.ValueType{wazeroapi.ValueTypeI64, wazeroapi.ValueTypeI64}, []wazeroapi.ValueType{wazeroapi.ValueTypeI64}).
		Export(vos.HostSandboxRead)

	// soi_sandbox_write(path_ptr:i64, path_len:i64, data_ptr:i64, data_len:i64) -> i32
	soiBuilder.NewFunctionBuilder().
		WithGoModuleFunction(wazeroapi.GoModuleFunc(func(ctx context.Context, mod wazeroapi.Module, stack []uint64) {
			pathPtr := uint32(stack[0])
			pathLen := uint32(stack[1])
			dataPtr := uint32(stack[2])
			dataLen := uint32(stack[3])
			path := readStrFromModule(mod, pathPtr, pathLen)
			data := readBytesFromModule(mod, dataPtr, dataLen)
			if err := host.SandboxWrite(path, data); err != nil {
				stack[0] = 1
			} else {
				stack[0] = 0
			}
		}), []wazeroapi.ValueType{wazeroapi.ValueTypeI64, wazeroapi.ValueTypeI64, wazeroapi.ValueTypeI64, wazeroapi.ValueTypeI64}, []wazeroapi.ValueType{wazeroapi.ValueTypeI32}).
		Export(vos.HostSandboxWrite)

	// soi_sandbox_list(path_ptr:i64, path_len:i64) -> i64
	soiBuilder.NewFunctionBuilder().
		WithGoModuleFunction(wazeroapi.GoModuleFunc(func(ctx context.Context, mod wazeroapi.Module, stack []uint64) {
			ptr := uint32(stack[0])
			length := uint32(stack[1])
			path := readStrFromModule(mod, ptr, length)
			names, err := host.SandboxList(path)
			var data []byte
			if err != nil {
				data = []byte(`{"error":"` + err.Error() + `"}`)
			} else {
				data, _ = json.Marshal(names)
			}
			stack[0] = writePackedToModule(mod, data)
		}), []wazeroapi.ValueType{wazeroapi.ValueTypeI64, wazeroapi.ValueTypeI64}, []wazeroapi.ValueType{wazeroapi.ValueTypeI64}).
		Export(vos.HostSandboxList)

	// soi_sandbox_stat(path_ptr:i64, path_len:i64) -> i64
	soiBuilder.NewFunctionBuilder().
		WithGoModuleFunction(wazeroapi.GoModuleFunc(func(ctx context.Context, mod wazeroapi.Module, stack []uint64) {
			ptr := uint32(stack[0])
			length := uint32(stack[1])
			path := readStrFromModule(mod, ptr, length)
			info, err := host.SandboxStat(path)
			var data []byte
			if err != nil {
				data = []byte(`{"error":"` + err.Error() + `"}`)
			} else {
				data, _ = json.Marshal(info)
			}
			stack[0] = writePackedToModule(mod, data)
		}), []wazeroapi.ValueType{wazeroapi.ValueTypeI64, wazeroapi.ValueTypeI64}, []wazeroapi.ValueType{wazeroapi.ValueTypeI64}).
		Export(vos.HostSandboxStat)

	// soi_sandbox_exec(cmd_ptr:i64, cmd_len:i64) -> i64
	soiBuilder.NewFunctionBuilder().
		WithGoModuleFunction(wazeroapi.GoModuleFunc(func(ctx context.Context, mod wazeroapi.Module, stack []uint64) {
			ptr := uint32(stack[0])
			length := uint32(stack[1])
			cmd := readStrFromModule(mod, ptr, length)
			result, err := host.SandboxExec(cmd)
			var data []byte
			if err != nil {
				data = []byte(`{"error":"` + err.Error() + `"}`)
			} else {
				data, _ = json.Marshal(result)
			}
			stack[0] = writePackedToModule(mod, data)
		}), []wazeroapi.ValueType{wazeroapi.ValueTypeI64, wazeroapi.ValueTypeI64}, []wazeroapi.ValueType{wazeroapi.ValueTypeI64}).
		Export(vos.HostSandboxExec)

	if _, err := soiBuilder.Instantiate(ctx); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("instantiate soi host module: %w", err)
	}

	// --- 3) Pre-compile the user module ---
	compiled, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("compile wasm module: %w", err)
	}

	return &StdioPlugin{
		wasmBytes:  wasmBytes,
		runtime:    rt,
		compiled:   compiled,
		sandboxDir: sandboxDir,
	}, nil
}

// Execute runs the plugin with the given tool name and arguments.
// The input {tool, args, sandbox_root} is serialized as JSON, written to
// stdin, and then the module _start is invoked. After completion, stdout
// is read and returned as the result.
func (p *StdioPlugin) Execute(ctx context.Context, toolName string, args map[string]interface{}) ([]byte, error) {
	req := map[string]interface{}{
		"tool":         toolName,
		"args":         args,
		"sandbox_root": p.sandboxDir,
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	stdinBuf := bytes.NewReader(append(reqJSON, '\n'))
	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	moduleName := fmt.Sprintf("stdio_plugin_%d", time.Now().UnixNano())
	modConfig := wazero.NewModuleConfig().
		WithName(moduleName).
		WithStdin(stdinBuf).
		WithStdout(stdoutBuf).
		WithStderr(stderrBuf).
		WithSysNanotime().
		WithSysWalltime().
		WithRandSource(rand.Reader)

	if p.sandboxDir != "" {
		modConfig = modConfig.WithFSConfig(wazero.NewFSConfig().WithDirMount(p.sandboxDir, "/sandbox"))
	}

	mod, err := p.runtime.InstantiateModule(ctx, p.compiled, modConfig)
	if err != nil {
		return nil, fmt.Errorf("instantiate module: %w", err)
	}
	if mod != nil {
		_ = mod.Close(ctx)
	}

	result := bytes.TrimSpace(stdoutBuf.Bytes())
	if len(result) == 0 {
		if stderr := stderrBuf.String(); stderr != "" {
			return nil, fmt.Errorf("plugin error: %s", stderr)
		}
		return nil, fmt.Errorf("no output from plugin")
	}
	return result, nil
}

// Close releases the underlying WASM runtime and compiled module.
func (p *StdioPlugin) Close() error {
	if p.runtime != nil {
		return p.runtime.Close(context.Background())
	}
	return nil
}

// --- helpers (memory read/write) ---

func readStrFromModule(mod wazeroapi.Module, ptr, length uint32) string {
	if mod == nil || length == 0 {
		return ""
	}
	data, ok := mod.Memory().Read(ptr, length)
	if !ok {
		return ""
	}
	return string(data)
}

func readBytesFromModule(mod wazeroapi.Module, ptr, length uint32) []byte {
	if mod == nil || length == 0 {
		return nil
	}
	data, ok := mod.Memory().Read(ptr, length)
	if !ok {
		return nil
	}
	out := make([]byte, len(data))
	copy(out, data)
	return out
}

func writePackedToModule(mod wazeroapi.Module, data []byte) uint64 {
	if mod == nil || mod.Memory() == nil {
		return 0
	}
	if uint32(len(data)) > vos.MemoryMaxOutputSize {
		data = data[:vos.MemoryMaxOutputSize]
	}
	const hostOutputOffset = uint32(65536)
	mod.Memory().Write(hostOutputOffset, data)
	return vos.Pack(hostOutputOffset, uint32(len(data)))
}
