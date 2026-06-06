package soi

import (
	"context"
	"fmt"

	"github.com/Source-of-Intelligence/soi-executor/pkg/engine"
	"github.com/Source-of-Intelligence/soi-vos"
)

// StdioPlugin wraps a standard Go WASM module (wasip1) that communicates
// via WASI stdin/stdout. It uses the wasm-executor engine with WASI enabled.
type StdioPlugin struct {
	executor   *engine.Executor
	moduleName string
	wasmBytes  []byte
}

// NewStdioPlugin creates a new StdioPlugin from WASM bytes.
func NewStdioPlugin(ctx context.Context, wasmBytes []byte, host vos.HostFunctions) (*StdioPlugin, error) {
	// For stdio plugins, we still need WASI but don't need SOI host functions
	// (they communicate via stdin/stdout, not host imports).
	// However, we register the SOI ABI anyway for consistency.
	abi := NewSOIABI(host)

	exec, err := engine.NewExecutor(ctx, &engine.ExecutorConfig{
		RuntimeConfig: &engine.RuntimeConfig{
			EnableWASI: true,
		},
		ABIs: []engine.ABI{abi},
	})
	if err != nil {
		return nil, fmt.Errorf("create executor: %w", err)
	}

	return &StdioPlugin{
		executor:   exec,
		moduleName: "stdio_plugin",
		wasmBytes:  wasmBytes,
	}, nil
}

// Execute runs a tool call on the stdio plugin by writing the request to stdin
// and reading the response from stdout.
func (p *StdioPlugin) Execute(ctx context.Context, toolName string, args map[string]interface{}) ([]byte, error) {
	// For stdio ABI, the plugin reads from stdin and writes to stdout.
	// The wasm-executor handles WASI stdin/stdout via ModuleConfig.
	// This is a simplified implementation; the full version would use
	// bytes.Reader/bytes.Buffer for stdin/stdout capture.
	//
	// TODO: Enhance wasm-executor engine to support stdin/stdout redirection
	// for WASI modules, similar to soi/internal/wasm/stdio_plugin.go.

	return nil, fmt.Errorf("stdio ABI execution not yet implemented in wasm-executor; use Export ABI (.soi) plugins")
}

// Close releases resources.
func (p *StdioPlugin) Close() error {
	return p.executor.Close()
}
