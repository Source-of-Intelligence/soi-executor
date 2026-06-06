// Package soi provides the SOI ABI implementation for wasm-executor.
// It enables SOI WASM plugins (compiled with soi-sdk) to be loaded and executed
// through the standard wasm-executor engine.
//
// The SOI ABI defines host functions (soi_log, soi_now, etc.) and an execution
// protocol (soi_init / soi_execute) that TinyGo-compiled SOI plugins export.
package soi

import (
	"context"
	"encoding/json"

	"github.com/tetratelabs/wazero/api"
	"soi.dev/soi-vos"
	"wasm-executor/pkg/types"
)

// SOIABI implements the types.ABI interface for SOI WASM plugins.
// It registers SOI host functions into the wasm-executor runtime.
type SOIABI struct {
	host vos.HostFunctions
}

// NewSOIABI creates a new SOIABI with the given host functions implementation.
func NewSOIABI(host vos.HostFunctions) *SOIABI {
	return &SOIABI{host: host}
}

func (s *SOIABI) Name() string      { return "soi" }
func (s *SOIABI) Version() string   { return vos.ABIVersion }
func (s *SOIABI) Modules() []string { return []string{vos.HostModuleName} }

// SetupHostFunctions registers all SOI host functions into the runtime.
func (s *SOIABI) SetupHostFunctions(ctx context.Context, runtime types.RuntimeInterface, moduleName string) error {
	if moduleName != vos.HostModuleName {
		return nil
	}

	// Use the underlying wazero runtime to register host functions with
	// explicit parameter/result types (required for api.GoModuleFunc).
	wazeroRT := runtime.GetWazeroRuntime()
	builder := wazeroRT.NewHostModuleBuilder(vos.HostModuleName)

	// soi_log(level:i32, ptr:i64, len:i64)
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(s.hostLog),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI64, api.ValueTypeI64}, nil).
		Export(vos.HostLog)

	// soi_now() → i64
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(s.hostNow),
			nil, []api.ValueType{api.ValueTypeI64}).
		Export(vos.HostNow)

	// soi_random(ptr:i64, len:i64)
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(s.hostRandom),
			[]api.ValueType{api.ValueTypeI64, api.ValueTypeI64}, nil).
		Export(vos.HostRandom)

	// soi_sandbox_read(path_ptr:i64, path_len:i64) → i64
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(s.hostSandboxRead),
			[]api.ValueType{api.ValueTypeI64, api.ValueTypeI64}, []api.ValueType{api.ValueTypeI64}).
		Export(vos.HostSandboxRead)

	// soi_sandbox_write(path_ptr:i64, path_len:i64, data_ptr:i64, data_len:i64) → i32
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(s.hostSandboxWrite),
			[]api.ValueType{api.ValueTypeI64, api.ValueTypeI64, api.ValueTypeI64, api.ValueTypeI64}, []api.ValueType{api.ValueTypeI32}).
		Export(vos.HostSandboxWrite)

	// soi_sandbox_list(path_ptr:i64, path_len:i64) → i64
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(s.hostSandboxList),
			[]api.ValueType{api.ValueTypeI64, api.ValueTypeI64}, []api.ValueType{api.ValueTypeI64}).
		Export(vos.HostSandboxList)

	// soi_sandbox_stat(path_ptr:i64, path_len:i64) → i64
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(s.hostSandboxStat),
			[]api.ValueType{api.ValueTypeI64, api.ValueTypeI64}, []api.ValueType{api.ValueTypeI64}).
		Export(vos.HostSandboxStat)

	// soi_sandbox_exec(cmd_ptr:i64, cmd_len:i64) → i64
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(s.hostSandboxExec),
			[]api.ValueType{api.ValueTypeI64, api.ValueTypeI64}, []api.ValueType{api.ValueTypeI64}).
		Export(vos.HostSandboxExec)

	_, err := builder.Instantiate(ctx)
	return err
}

// --- Host function implementations (wazero GoModuleFunc signatures) ---

// soi_log(level:i32, ptr:i64, len:i64)
func (s *SOIABI) hostLog(ctx context.Context, mod api.Module, stack []uint64) {
	level := int32(stack[0])
	msg := readStr(mod, uint32(stack[1]), uint32(stack[2]))
	s.host.Log(level, msg)
}

// soi_now() → i64
func (s *SOIABI) hostNow(ctx context.Context, mod api.Module, stack []uint64) {
	stack[0] = uint64(s.host.Now())
}

// soi_random(ptr:i64, len:i64)
func (s *SOIABI) hostRandom(ctx context.Context, mod api.Module, stack []uint64) {
	ptr := uint32(stack[0])
	length := uint32(stack[1])
	if length > 0 {
		buf := make([]byte, length)
		_ = s.host.Random(buf)
		mod.Memory().Write(ptr, buf)
	}
}

// soi_sandbox_read(path_ptr:i64, path_len:i64) → i64
func (s *SOIABI) hostSandboxRead(ctx context.Context, mod api.Module, stack []uint64) {
	pathPtr := uint32(stack[0])
	pathLen := uint32(stack[1])
	path := readStr(mod, pathPtr, pathLen)
	data, err := s.host.SandboxRead(path)
	if err != nil {
		data = []byte(`{"error":"` + err.Error() + `"}`)
	}
	stack[0] = writePacked(mod, data)
}

// soi_sandbox_write(path_ptr:i64, path_len:i64, data_ptr:i64, data_len:i64) → i32
func (s *SOIABI) hostSandboxWrite(ctx context.Context, mod api.Module, stack []uint64) {
	path := readStr(mod, uint32(stack[0]), uint32(stack[1]))
	data := readRaw(mod, uint32(stack[2]), uint32(stack[3]))
	if err := s.host.SandboxWrite(path, data); err != nil {
		stack[0] = 1
	} else {
		stack[0] = 0
	}
}

// soi_sandbox_list(path_ptr:i64, path_len:i64) → i64
func (s *SOIABI) hostSandboxList(ctx context.Context, mod api.Module, stack []uint64) {
	path := readStr(mod, uint32(stack[0]), uint32(stack[1]))
	names, err := s.host.SandboxList(path)
	var data []byte
	if err != nil {
		data = []byte(`{"error":"` + err.Error() + `"}`)
	} else {
		data, _ = json.Marshal(names)
	}
	stack[0] = writePacked(mod, data)
}

// soi_sandbox_stat(path_ptr:i64, path_len:i64) → i64
func (s *SOIABI) hostSandboxStat(ctx context.Context, mod api.Module, stack []uint64) {
	path := readStr(mod, uint32(stack[0]), uint32(stack[1]))
	info, err := s.host.SandboxStat(path)
	var data []byte
	if err != nil {
		data = []byte(`{"error":"` + err.Error() + `"}`)
	} else {
		data, _ = json.Marshal(info)
	}
	stack[0] = writePacked(mod, data)
}

// soi_sandbox_exec(cmd_ptr:i64, cmd_len:i64) → i64
func (s *SOIABI) hostSandboxExec(ctx context.Context, mod api.Module, stack []uint64) {
	cmd := readStr(mod, uint32(stack[0]), uint32(stack[1]))
	result, err := s.host.SandboxExec(cmd)
	var data []byte
	if err != nil {
		data = []byte(`{"error":"` + err.Error() + `"}`)
	} else {
		data, _ = json.Marshal(result)
	}
	stack[0] = writePacked(mod, data)
}

// --- Memory helpers ---

func readStr(mod api.Module, ptr, length uint32) string {
	if length == 0 || mod == nil {
		return ""
	}
	data, ok := mod.Memory().Read(ptr, length)
	if !ok {
		return ""
	}
	return string(data)
}

func readRaw(mod api.Module, ptr, length uint32) []byte {
	if length == 0 || mod == nil {
		return nil
	}
	data, ok := mod.Memory().Read(ptr, length)
	if !ok {
		return nil
	}
	result := make([]byte, len(data))
	copy(result, data)
	return result
}

func writePacked(mod api.Module, data []byte) uint64 {
	if mod == nil || mod.Memory() == nil {
		return 0
	}
	if uint32(len(data)) > vos.MemoryMaxOutputSize {
		data = data[:vos.MemoryMaxOutputSize]
	}
	// Use a high fixed address (64KB) to avoid conflicting with TinyGo runtime
	// which uses lower addresses for heap/stack.
	const hostOutputOffset = uint32(65536) // 64KB
	mem := mod.Memory()
	mem.Write(hostOutputOffset, data)
	return vos.Pack(hostOutputOffset, uint32(len(data)))
}
