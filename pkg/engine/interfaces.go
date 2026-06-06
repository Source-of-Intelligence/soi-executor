// pkg/engine/interfaces.go
// engine包导出types包的别名，保持向后兼容

package engine

import (
	"github.com/Source-of-Intelligence/soi-executor/pkg/types"
)

// RuntimeInterface 运行时接口，供ABI和Context包使用
type RuntimeInterface = types.RuntimeInterface

// ABI 抽象接口，定义不同ABI的实现
type ABI = types.ABI
