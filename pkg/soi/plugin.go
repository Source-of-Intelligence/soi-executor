package soi

import (
	"context"
	"encoding/json"
	"fmt"

	"soi.dev/soi-vos"
	"wasm-executor/pkg/engine"
)

// ExecutionRequest is the JSON payload passed to soi_execute.
type ExecutionRequest struct {
	Tool        string          `json:"tool"`
	Args        json.RawMessage `json:"args"`
	SandboxRoot string          `json:"sandbox_root,omitempty"`
}

// SOIPlugin wraps a compiled SOI WASM module with the wasm-executor engine.
// It handles loading, initialization (soi_init), and tool execution (soi_execute).
type SOIPlugin struct {
	executor   *engine.Executor
	moduleName string
}

// NewSOIPlugin creates a new SOIPlugin from WASM bytes.
// The host parameter provides the HostFunctions implementation
// (use vos.MockHost for testing, or a real implementation for production).
func NewSOIPlugin(ctx context.Context, wasmBytes []byte, host vos.HostFunctions) (*SOIPlugin, error) {
	abi := NewSOIABI(host)

	exec, err := engine.NewExecutor(ctx, &engine.ExecutorConfig{
		RuntimeConfig: &engine.RuntimeConfig{
			EnableWASI:   true,
			NoOpProcExit: true, // TinyGo Export ABI: 阻止 proc_exit(0) 关闭模块
		},
		ABIs: []engine.ABI{abi},
	})
	if err != nil {
		return nil, fmt.Errorf("create executor: %w", err)
	}

	moduleName := "soi_plugin"

	if err := exec.LoadModule(ctx, moduleName, wasmBytes); err != nil {
		exec.Close()
		return nil, fmt.Errorf("load module: %w", err)
	}

	// 调用 soi_init 注册工具
	mod, ok := exec.GetRuntime().GetModule(moduleName)
	if !ok {
		exec.Close()
		return nil, fmt.Errorf("module %s not instantiated after load", moduleName)
	}

	initFn := mod.ExportedFunction(vos.ExportInit)
	if initFn != nil {
		_, err := initFn.Call(ctx)
		if err != nil {
			exec.Close()
			return nil, fmt.Errorf("soi_init failed: %w", err)
		}
	}

	return &SOIPlugin{executor: exec, moduleName: moduleName}, nil
}

// Execute runs a tool call on the SOI plugin via the export ABI (soi_execute).
func (p *SOIPlugin) Execute(ctx context.Context, toolName string, args map[string]interface{}) ([]byte, error) {
	req := ExecutionRequest{
		Tool: toolName,
		Args: mustMarshal(args),
	}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Get the module instance
	mod, ok := p.executor.GetRuntime().GetModule(p.moduleName)
	if !ok {
		return nil, fmt.Errorf("module %s not instantiated", p.moduleName)
	}

	// Write request to WASM linear memory
	inputPtr := uint32(vos.MemoryReservedSize)
	mod.Memory().Write(inputPtr, reqJSON)

	// Call soi_execute(ptr, length)
	soiExec := mod.ExportedFunction(vos.ExportExecute)
	if soiExec == nil {
		return nil, fmt.Errorf("function %s not found in module", vos.ExportExecute)
	}

	results, err := soiExec.Call(ctx, uint64(inputPtr), uint64(len(reqJSON)))
	if err != nil {
		return nil, fmt.Errorf("wasm execute: %w", err)
	}

	// Parse packed result: high 32 bits = ptr, low 32 bits = length
	resultPtr, resultLen := vos.Unpack(results[0])

	if resultLen == 0 {
		return nil, nil
	}

	data, ok := mod.Memory().Read(resultPtr, resultLen)
	if !ok {
		return nil, fmt.Errorf("failed to read result from wasm memory")
	}

	// Copy to avoid dangling reference
	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

// ExecuteJSON runs a tool call with pre-marshaled args JSON.
func (p *SOIPlugin) ExecuteJSON(ctx context.Context, toolName string, argsJSON json.RawMessage) ([]byte, error) {
	var args map[string]interface{}
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	return p.Execute(ctx, toolName, args)
}

// Close releases the underlying executor resources.
func (p *SOIPlugin) Close() error {
	return p.executor.Close()
}

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil || data == nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}
