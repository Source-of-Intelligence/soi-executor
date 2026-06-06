// pkg/types/interfaces.go
// 共享接口定义，避免循环依赖

package types

import (
	"context"

	"github.com/tetratelabs/wazero"
)

// RuntimeInterface 运行时接口，供ABI和Context包使用
type RuntimeInterface interface {
	// RegisterHostFunction 注册单个主机函数
	RegisterHostFunction(moduleName, funcName string, fn interface{}) error
	// RegisterHostFunctions 批量注册主机函数到同一模块
	RegisterHostFunctions(moduleName string, functions map[string]interface{}) error
	// InstantiateHostModule 实例化指定名称的主机模块
	InstantiateHostModule(moduleName string) error
	// GetWazeroRuntime 获取底层wazero运行时
	GetWazeroRuntime() wazero.Runtime
}

// ABI 抽象接口，定义不同ABI的实现
type ABI interface {
	// Name 返回ABI名称
	Name() string
	// Version 返回ABI版本
	Version() string
	// Modules 返回此ABI处理的模块名称列表
	Modules() []string
	// SetupHostFunctions 设置主机函数到运行时
	SetupHostFunctions(ctx context.Context, runtime RuntimeInterface, moduleName string) error
}
