# soi-executor — Pure WASM Execution Engine

基于 [wazero](https://github.com/tetratelabs/wazero) 的纯 WASM 执行引擎，为 SOI 生态提供 WASM 模块加载、实例化和执行能力。

## 定位

```
soi-vos （契约层）
    ↑
wasm-executor （WASM 执行引擎，依赖 soi-vos）
    ↑
soi-sdk / soi （使用 wasm-executor 加载和执行 WASM 插件）
```

wasm-executor **只负责 WASM 执行**，不包含任何插件开发 SDK 功能（脚手架、代码生成等已移至 soi-sdk）。

## 核心功能

| 功能 | 说明 |
|------|------|
| **WASM 运行时** | 基于 wazero 的 WASM 模块编译、实例化、执行 |
| **SOI ABI** | SOI 插件的 Export ABI 实现（`soi_init` / `soi_execute`） |
| **Stdio ABI** | 标准 WASI 的 stdio 通信 ABI |
| **Host Functions** | 注册 soi-vos 定义的 host functions 到 WASM 模块 |
| **Context 注入** | 将 Go context 传递到 WASM 模块 |

## 项目结构

```
wasm-executor/
├── pkg/
│   ├── engine/          # 核心 WASM 执行引擎
│   │   ├── runtime.go   # wazero 运行时封装
│   │   ├── executor.go  # 高级执行器（模块管理、ABI 路由）
│   │   └── interfaces.go # 引擎接口定义
│   ├── soi/             # SOI ABI 实现
│   │   ├── plugin.go    # SOIPlugin（Export ABI）
│   │   ├── stdio.go     # StdioPlugin（Stdio ABI）
│   │   └── abi.go       # SOIABI（host functions 注册）
│   ├── abi/             # 通用 ABI 扩展系统
│   │   ├── abi.go       # ABI 接口和管理器
│   │   ├── custom.go    # 自定义 ABI
│   │   └── blockchain.go # 区块链 ABI
│   ├── context/         # Context 注入机制
│   │   └── injector.go  # Context 注入器
│   └── types/           # 类型接口
│       └── interfaces.go
├── cmd/
│   └── executor/        # WASM 执行器 CLI
├── examples/            # 执行器使用示例
│   ├── simple/          # 基础 WASM 执行
│   ├── blockchain/      # 区块链 ABI 示例
│   └── custom-abi/      # 自定义 ABI 示例
├── go.mod
└── README.md
```

## 快速开始

### 安装

```bash
go get wasm-executor
```

### 基础使用

```go
package main

import (
    "context"
    "wasm-executor/pkg/engine"
)

func main() {
    ctx := context.Background()

    // 创建执行器
    executor, err := engine.NewExecutor(ctx, &engine.ExecutorConfig{
        RuntimeConfig: &engine.RuntimeConfig{
            EnableWASI: true,
        },
    })
    if err != nil {
        panic(err)
    }
    defer executor.Close()

    // 加载 WASM 模块
    wasmBytes := []byte{...} // WASM 字节码
    if err := executor.LoadModule(ctx, "my_module", wasmBytes); err != nil {
        panic(err)
    }

    // 执行函数
    result, err := executor.Execute("my_module", "my_function", 1, 2, 3)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Result: %v\n", result.Values)
}
```

### 加载 SOI 插件（Export ABI）

```go
import (
    "context"
    "wasm-executor/pkg/soi"
    "github.com/Source-of-Intelligence/soi-vos"
)

func main() {
    ctx := context.Background()

    // 创建 host（实现 soi-vos.HostFunctions）
    host := vos.NewMockHost(nil)

    // 创建 SOI 插件
    plugin, err := soi.NewSOIPlugin(ctx, wasmBytes, host)
    if err != nil {
        panic(err)
    }

    // 执行工具
    result, err := plugin.Execute(ctx, "my_tool", map[string]interface{}{
        "input": "hello",
    })
}
```

### 使用自定义 ABI

```go
import "wasm-executor/pkg/abi"

// 创建自定义 ABI
customABI := abi.NewCustomABIBuilder("my_abi", "1.0.0").
    WithModule("env").
    WithHostFunction("my_function", func(ctx context.Context, x int32) int32 {
        return x * 2
    }).
    Build()

// 注册到执行器
executor.RegisterABI(customABI)
```

### 使用执行器 CLI

```bash
# 执行 WASM 文件
go run cmd/executor/main.go -wasm app.wasm -func main -wasi

# 启用区块链 ABI
go run cmd/executor/main.go -wasm contract.wasm -func get_value -blockchain
```

## API 文档

### Engine 包

| 函数/方法 | 说明 |
|-----------|------|
| `NewRuntime(ctx, config)` | 创建 WASM 运行时 |
| `NewExecutor(ctx, config)` | 创建高级执行器 |
| `Runtime.CompileModule(name, wasmBytes)` | 编译 WASM 模块 |
| `Runtime.InstantiateModule(name, config)` | 实例化模块 |
| `Runtime.ExecuteFunction(module, func, params...)` | 执行函数 |
| `Runtime.RegisterHostFunction(module, name, fn)` | 注册主机函数 |

### SOI 包

| 函数/方法 | 说明 |
|-----------|------|
| `NewSOIPlugin(ctx, wasmBytes, host)` | 创建 SOI 插件（Export ABI） |
| `SOIPlugin.Execute(ctx, toolName, args)` | 执行插件工具 |
| `NewStdioPlugin(ctx, wasmBytes, host, info)` | 创建 Stdio 插件 |
| `StdioPlugin.Execute(ctx, toolName, args)` | 执行 stdio 工具 |

### ABI 包

| 函数/方法 | 说明 |
|-----------|------|
| `NewManager()` | 创建 ABI 管理器 |
| `Manager.Register(abi)` | 注册 ABI |
| `NewCustomABI(name, version, modules)` | 创建自定义 ABI |
| `NewBlockchainABI(ctx)` | 创建区块链 ABI |

## 测试

```bash
# 运行所有测试
go test ./...

# 运行特定包测试
go test ./pkg/engine
go test ./pkg/abi
go test ./pkg/soi
```

## 扩展

### 添加新的 ABI 类型

实现 `abi.ABI` 接口：

```go
type MyABI struct{}

func (a *MyABI) Name() string { return "my_abi" }
func (a *MyABI) Version() string { return "1.0.0" }
func (a *MyABI) Modules() []string { return []string{"my_module"} }

func (a *MyABI) SetupHostFunctions(ctx context.Context, runtime *engine.Runtime, moduleName string) error {
    return runtime.RegisterHostFunction(moduleName, "my_func", myHostFunc)
}
```

## 依赖

| 依赖 | 用途 |
|------|------|
| `github.com/tetratelabs/wazero` | WASM 运行时 |
| `github.com/Source-of-Intelligence/soi-vos` | SOI 契约层（HostFunctions 接口、ABI 常量） |

## 许可证

MIT
