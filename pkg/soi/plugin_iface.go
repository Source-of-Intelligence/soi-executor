package soi

import "context"

// Plugin is the unified interface for both SOI export-ABI plugins and
// WASI stdio plugins. Both implementations are runtime-compatible and
// can be managed interchangeably by callers such as soi-server.
type Plugin interface {
	// Execute runs the plugin with the given tool name and arguments,
	// returning the raw output (typically JSON) or an error.
	Execute(ctx context.Context, toolName string, args map[string]interface{}) ([]byte, error)

	// Close releases underlying WASM resources (runtime, compiled module, etc.).
	Close() error
}

// Ensure both plugin types implement Plugin at compile time.
var _ Plugin = (*SOIPlugin)(nil)
var _ Plugin = (*StdioPlugin)(nil)
