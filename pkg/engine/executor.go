// pkg/engine/executor.go
// 高级执行器，支持ABI管理和Context注入

package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/Source-of-Intelligence/soi-executor/pkg/abi"
	wasmctx "github.com/Source-of-Intelligence/soi-executor/pkg/context"
	"github.com/tetratelabs/wazero"
)

// Executor 高级WASM执行器
type Executor struct {
	runtime *Runtime
	// ABI管理器
	abiManager *abi.Manager
	// Context注入器
	ctxInjector *wasmctx.Injector
}

// ExecutorConfig 执行器配置
type ExecutorConfig struct {
	RuntimeConfig *RuntimeConfig
	// 预加载的ABI列表
	ABIs []ABI
}

// NewExecutor 创建新的执行器
func NewExecutor(ctx context.Context, config *ExecutorConfig) (*Executor, error) {
	if config == nil {
		config = &ExecutorConfig{}
	}

	if config.RuntimeConfig == nil {
		config.RuntimeConfig = &RuntimeConfig{}
	}

	// 创建运行时
	rt, err := NewRuntime(ctx, config.RuntimeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create runtime: %w", err)
	}

	// 创建ABI管理器
	abiManager := abi.NewManager()

	// 注册预加载的ABI
	for _, a := range config.ABIs {
		if err := abiManager.Register(a); err != nil {
			return nil, fmt.Errorf("failed to register ABI %s: %w", a.Name(), err)
		}
	}

	// 创建Context注入器
	ctxInjector := wasmctx.NewInjector()

	executor := &Executor{
		runtime:     rt,
		abiManager:  abiManager,
		ctxInjector: ctxInjector,
	}

	return executor, nil
}

// LoadModule 加载并实例化WASM模块（自动应用已注册的ABI）
func (e *Executor) LoadModule(ctx context.Context, name string, wasmBytes []byte, abis ...string) error {
	// 编译模块
	compiled, err := e.runtime.CompileModule(name, wasmBytes)
	if err != nil {
		return err
	}

	// 获取模块导入信息
	imports := compiled.ImportedFunctions()

	// 为需要的ABI注册主机函数（每个模块只处理一次）
	seenModules := make(map[string]bool)
	for _, imp := range imports {
		moduleName, _, isImport := imp.Import()
		if !isImport || seenModules[moduleName] {
			continue
		}
		seenModules[moduleName] = true

		// 检查是否有ABI可以处理这个模块
		if a, ok := e.abiManager.GetByModule(moduleName); ok {
			if err := a.SetupHostFunctions(ctx, e.runtime, moduleName); err != nil {
				return fmt.Errorf("failed to setup ABI %s: %w", a.Name(), err)
			}
		}
	}

	// 配置模块
	moduleConfig := wazero.NewModuleConfig()

	// 应用Context注入
	if err := e.ctxInjector.ApplyToModule(ctx, e.runtime, moduleConfig); err != nil {
		return fmt.Errorf("failed to apply context injection: %w", err)
	}

	// 实例化模块
	_, err = e.runtime.InstantiateModule(name, moduleConfig)
	if err != nil {
		// WASI 模块的 _start 可能调用 proc_exit(0)，这是正常行为（TinyGo Export ABI）
		// 模块的内存和导出函数仍然可用
		if !isProcExitError(err) {
			return err
		}
	}

	return nil
}

// Execute 执行指定模块的函数
func (e *Executor) Execute(moduleName, funcName string, params ...uint64) (*ExecutionResult, error) {
	return e.runtime.ExecuteFunction(moduleName, funcName, params...)
}

// ExecuteWithContext 使用指定Context执行函数
func (e *Executor) ExecuteWithContext(ctx context.Context, moduleName, funcName string, params ...uint64) (*ExecutionResult, error) {
	// 临时切换context执行
	module, ok := e.runtime.GetModule(moduleName)
	if !ok {
		return nil, fmt.Errorf("module %s not found", moduleName)
	}

	fn := module.ExportedFunction(funcName)
	if fn == nil {
		return nil, fmt.Errorf("function %s not found in module %s", funcName, moduleName)
	}

	result := &ExecutionResult{}
	values, err := fn.Call(ctx, params...)
	if err != nil {
		result.Error = err
		return result, err
	}

	result.Values = values
	return result, nil
}

// RegisterABI 注册新的ABI
func (e *Executor) RegisterABI(a ABI) error {
	return e.abiManager.Register(a)
}

// GetRuntime 获取底层运行时
func (e *Executor) GetRuntime() *Runtime {
	return e.runtime
}

// GetABIManager 获取ABI管理器
func (e *Executor) GetABIManager() *abi.Manager {
	return e.abiManager
}

// Close 关闭执行器
func (e *Executor) Close() error {
	return e.runtime.Close()
}

// isProcExitError checks if the error is a WASI proc_exit with code 0.
// TinyGo-compiled WASM modules call proc_exit(0) after main() returns,
// which is normal for Export ABI plugins — the module's memory and exports
// remain accessible.
func isProcExitError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "exit_code(0)")
}
