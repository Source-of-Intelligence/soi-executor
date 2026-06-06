// pkg/engine/runtime.go
// 核心WASM运行时引擎，基于wazero封装

package engine

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// RuntimeConfig 运行时配置
type RuntimeConfig struct {
	// 是否启用WASI
	EnableWASI bool
	// 内存限制 (页数, 每页64KB)
	MemoryLimitPages uint32
	// CPU周期限制 (0表示无限制)
	CPUCycleLimit uint64
	// 自定义主机函数
	HostFunctions map[string]HostFunction
	// 跳过WASM模块的_start函数（用于Export ABI插件，避免TinyGo proc_exit关闭模块）
	SkipStartFunction bool
	// 覆盖WASI proc_exit为no-op（用于TinyGo Export ABI插件）
	NoOpProcExit bool
}

// HostFunction 主机函数定义
type HostFunction struct {
	ModuleName string
	Name       string
	Signature  api.FunctionDefinition
	Fn         interface{}
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	// 返回值
	Values []uint64
	// 使用的CPU周期
	CPUCycles uint64
	// 使用的内存 (字节)
	MemoryUsage uint64
	// 错误信息
	Error error
}

// Runtime WASM运行时封装
type Runtime struct {
	ctx    context.Context
	config *RuntimeConfig
	// wazero运行时
	wazeroRuntime wazero.Runtime
	// 已编译模块缓存
	compiledModules map[string]wazero.CompiledModule
	// 已实例化模块
	modules map[string]api.Module
	// 已注册的主机模块构建器
	hostModuleBuilders map[string]wazero.HostModuleBuilder
	// 已实例化的主机模块
	hostModules map[string]api.Module
}

// NewRuntime 创建新的WASM运行时
func NewRuntime(ctx context.Context, config *RuntimeConfig) (*Runtime, error) {
	if config == nil {
		config = &RuntimeConfig{}
	}

	// Pre-allocate max memory to avoid runtime memory.grow,
	// which would invalidate TinyGo's unsafe.Pointer references.
	rConfig := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(64).       // 64 pages = 4MB max
		WithMemoryCapacityFromMax(true) // eagerly allocate, avoid re-allocations
	rt := wazero.NewRuntimeWithConfig(ctx, rConfig)

	r := &Runtime{
		ctx:                ctx,
		config:             config,
		wazeroRuntime:      rt,
		compiledModules:    make(map[string]wazero.CompiledModule),
		modules:            make(map[string]api.Module),
		hostModuleBuilders: make(map[string]wazero.HostModuleBuilder),
		hostModules:        make(map[string]api.Module),
	}

	// 如果启用WASI，导入WASI快照
	if config.EnableWASI {
		if config.NoOpProcExit {
			// 使用 NewFunctionExporter 导出所有 WASI 函数，
			// 然后覆盖 proc_exit 为 no-op。
			// 这对于 TinyGo Export ABI 插件是必需的，因为 TinyGo 的 _start
			// 在 main() 返回后会调用 proc_exit(0)，导致模块被 wazero 关闭。
			wasiBuilder := rt.NewHostModuleBuilder(wasi_snapshot_preview1.ModuleName)
			wasi_snapshot_preview1.NewFunctionExporter().ExportFunctions(wasiBuilder)
			// 覆盖 proc_exit 为 no-op（阻止模块被关闭）
			wasiBuilder.NewFunctionBuilder().
				WithFunc(func(ctx context.Context, exitCode uint32) {
					// no-op: 不退出模块，让导出函数继续可用
				}).
				Export("proc_exit")
			_, err := wasiBuilder.Instantiate(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to instantiate WASI with no-op proc_exit: %w", err)
			}
		} else {
			if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
				return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
			}
		}
	}

	return r, nil
}

// CompileModule 编译WASM模块
func (r *Runtime) CompileModule(name string, wasmBytes []byte) (wazero.CompiledModule, error) {
	compiled, err := r.wazeroRuntime.CompileModule(r.ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to compile module %s: %w", name, err)
	}

	r.compiledModules[name] = compiled
	return compiled, nil
}

// InstantiateModule 实例化已编译的模块
func (r *Runtime) InstantiateModule(name string, config wazero.ModuleConfig) (api.Module, error) {
	compiled, ok := r.compiledModules[name]
	if !ok {
		return nil, fmt.Errorf("module %s not compiled", name)
	}

	if config == nil {
		config = wazero.NewModuleConfig()
	}

	module, err := r.wazeroRuntime.InstantiateModule(r.ctx, compiled, config)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate module %s: %w", name, err)
	}

	r.modules[name] = module
	return module, nil
}

// ExecuteFunction 执行模块中的函数
func (r *Runtime) ExecuteFunction(moduleName, funcName string, params ...uint64) (*ExecutionResult, error) {
	module, ok := r.modules[moduleName]
	if !ok {
		return nil, fmt.Errorf("module %s not instantiated", moduleName)
	}

	fn := module.ExportedFunction(funcName)
	if fn == nil {
		return nil, fmt.Errorf("function %s not found in module %s", funcName, moduleName)
	}

	result := &ExecutionResult{}

	// 执行函数
	values, err := fn.Call(r.ctx, params...)
	if err != nil {
		result.Error = err
		return result, err
	}

	result.Values = values
	return result, nil
}

// GetModule 获取已实例化的模块
func (r *Runtime) GetModule(name string) (api.Module, bool) {
	m, ok := r.modules[name]
	return m, ok
}

// RegisterHostFunction 注册主机函数到运行时
// 注意：如果同一个模块需要注册多个函数，应该先调用 RegisterHostFunction 注册所有函数，
// 然后再加载依赖该模块的 WASM 模块。或者使用 RegisterHostFunctions 批量注册。
func (r *Runtime) RegisterHostFunction(moduleName, funcName string, fn interface{}) error {
	// 检查是否已经实例化了该主机模块
	if hostModule, ok := r.hostModules[moduleName]; ok {
		// 如果模块已经实例化，检查函数是否已存在
		if hostModule.ExportedFunction(funcName) != nil {
			// 函数已存在，返回成功（幂等）
			return nil
		}
		// 已经实例化的模块不能再添加函数，返回错误
		return fmt.Errorf("host module %s already instantiated, cannot add new function %s", moduleName, funcName)
	}

	// 获取或创建主机模块构建器
	builder, ok := r.hostModuleBuilders[moduleName]
	if !ok {
		builder = r.wazeroRuntime.NewHostModuleBuilder(moduleName)
		r.hostModuleBuilders[moduleName] = builder
	}

	// 添加函数到构建器
	builder.NewFunctionBuilder().WithFunc(fn).Export(funcName)

	return nil
}

// InstantiateHostModule 实例化指定名称的主机模块
// 在注册完所有主机函数后调用此方法
func (r *Runtime) InstantiateHostModule(moduleName string) error {
	builder, ok := r.hostModuleBuilders[moduleName]
	if !ok {
		// 没有该模块的构建器，可能已经被实例化或从未注册
		if _, exists := r.hostModules[moduleName]; exists {
			return nil // 已经实例化
		}
		return fmt.Errorf("no host functions registered for module %s", moduleName)
	}

	module, err := builder.Instantiate(r.ctx)
	if err != nil {
		return fmt.Errorf("failed to instantiate host module %s: %w", moduleName, err)
	}

	r.hostModules[moduleName] = module
	delete(r.hostModuleBuilders, moduleName)
	return nil
}

// RegisterHostFunctions 批量注册主机函数并立即实例化模块
func (r *Runtime) RegisterHostFunctions(moduleName string, functions map[string]interface{}) error {
	// 检查是否已经实例化
	if _, ok := r.hostModules[moduleName]; ok {
		return nil // 已经实例化，幂等返回
	}

	// 复用已有的 builder（可能已经通过 RegisterHostFunction 添加了函数）
	builder, ok := r.hostModuleBuilders[moduleName]
	if !ok {
		builder = r.wazeroRuntime.NewHostModuleBuilder(moduleName)
	}

	for funcName, fn := range functions {
		builder.NewFunctionBuilder().WithFunc(fn).Export(funcName)
	}

	module, err := builder.Instantiate(r.ctx)
	if err != nil {
		return fmt.Errorf("failed to instantiate host module %s: %w", moduleName, err)
	}

	r.hostModules[moduleName] = module
	delete(r.hostModuleBuilders, moduleName)
	return nil
}

// Close 关闭运行时并释放资源
func (r *Runtime) Close() error {
	return r.wazeroRuntime.Close(context.Background())
}

// GetWazeroRuntime 获取底层wazero运行时 (供高级扩展使用)
func (r *Runtime) GetWazeroRuntime() wazero.Runtime {
	return r.wazeroRuntime
}
