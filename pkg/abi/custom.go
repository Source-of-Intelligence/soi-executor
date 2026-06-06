// pkg/abi/custom.go
// 自定义ABI实现

package abi

import (
	"context"
	"fmt"

	"wasm-executor/pkg/types"
)

// CustomABI 自定义ABI实现
type CustomABI struct {
	name      string
	version   string
	modules   []string
	hostFuncs map[string]interface{}
}

// NewCustomABI 创建新的自定义ABI
func NewCustomABI(name, version string, modules []string) *CustomABI {
	return &CustomABI{
		name:      name,
		version:   version,
		modules:   modules,
		hostFuncs: make(map[string]interface{}),
	}
}

// Name 返回ABI名称
func (c *CustomABI) Name() string {
	return c.name
}

// Version 返回ABI版本
func (c *CustomABI) Version() string {
	return c.version
}

// Modules 返回此ABI处理的模块名称列表
func (c *CustomABI) Modules() []string {
	return c.modules
}

// RegisterHostFunction 注册主机函数
func (c *CustomABI) RegisterHostFunction(funcName string, fn interface{}) {
	c.hostFuncs[funcName] = fn
}

// SetupHostFunctions 设置主机函数到运行时
func (c *CustomABI) SetupHostFunctions(ctx context.Context, runtime types.RuntimeInterface, moduleName string) error {
	if len(c.hostFuncs) == 0 {
		return nil
	}

	// 使用批量注册
	if err := runtime.RegisterHostFunctions(moduleName, c.hostFuncs); err != nil {
		return fmt.Errorf("failed to register host functions for module %s: %w", moduleName, err)
	}
	return nil
}

// CustomABIBuilder 自定义ABI构建器
type CustomABIBuilder struct {
	abi *CustomABI
}

// NewCustomABIBuilder 创建自定义ABI构建器
func NewCustomABIBuilder(name, version string) *CustomABIBuilder {
	return &CustomABIBuilder{
		abi: NewCustomABI(name, version, []string{"env"}),
	}
}

// WithModule 添加模块名
func (b *CustomABIBuilder) WithModule(module string) *CustomABIBuilder {
	b.abi.modules = append(b.abi.modules, module)
	return b
}

// WithHostFunction 添加主机函数
func (b *CustomABIBuilder) WithHostFunction(name string, fn interface{}) *CustomABIBuilder {
	b.abi.RegisterHostFunction(name, fn)
	return b
}

// Build 构建自定义ABI
func (b *CustomABIBuilder) Build() types.ABI {
	return b.abi
}
