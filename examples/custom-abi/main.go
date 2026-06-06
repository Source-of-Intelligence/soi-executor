// examples/custom-abi/main.go
// 自定义ABI扩展示例 - 使用内嵌WASM字节码验证自定义ABI

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Source-of-Intelligence/soi-executor/pkg/abi"
	"github.com/Source-of-Intelligence/soi-executor/pkg/engine"
	"github.com/tetratelabs/wazero/api"
)

// testCustomWASM 是一个简单的WASM模块，导入 "env" 模块的自定义函数
//
// WAT:
// (module
//
//	(import "env" "custom_add" (func $custom_add (param i32 i32) (result i32)))
//	(import "env" "custom_multiply" (func $custom_multiply (param i32 i32) (result i32)))
//
//	(func (export "add_and_double") (param i32 i32) (result i32)
//	  (i32.mul
//	    (call $custom_add (local.get 0) (local.get 1))
//	    (i32.const 2)
//	  )
//	)
//
//	(func (export "multiply_result") (param i32 i32) (result i32)
//	  (call $custom_multiply (local.get 0) (local.get 1))
//	)
//
// )
var testCustomWASM = []byte{
	// WASM header
	0x00, 0x61, 0x73, 0x6d,
	0x01, 0x00, 0x00, 0x00,

	// Type section (id=1): 1 type
	0x01, 0x07, 0x01, // section id=1, size=7, 1 type
	0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f, // (i32, i32) -> i32

	// Import section (id=2): 2 imports
	0x02, 0x1a, 0x02, // section id=2, size=26, 2 imports
	// import 0: "env"."custom_add" -> type 0
	0x03, 0x65, 0x6e, 0x76, // "env"
	0x0a, 0x63, 0x75, 0x73, 0x74, 0x6f, 0x6d, 0x5f, 0x61, 0x64, 0x64, // "custom_add"
	0x00, 0x00, // kind=func, type index=0
	// import 1: "env"."custom_multiply" -> type 0
	0x03, 0x65, 0x6e, 0x76, // "env"
	0x0f, 0x63, 0x75, 0x73, 0x74, 0x6f, 0x6d, 0x5f, 0x6d, 0x75, 0x6c, 0x74, 0x69, 0x70, 0x6c, 0x79, // "custom_multiply"
	0x00, 0x00, // kind=func, type index=0

	// Function section (id=3): 2 functions
	0x03, 0x03, 0x02, // section id=3, size=3, 2 functions
	0x00, // func 2: type 0
	0x00, // func 3: type 0

	// Export section (id=7): 2 exports
	0x07, 0x24, 0x02, // section id=7, size=36, 2 exports
	// export "add_and_double"
	0x0e, 0x61, 0x64, 0x64, 0x5f, 0x61, 0x6e, 0x64, 0x5f, 0x64, 0x6f, 0x75, 0x62, 0x6c, 0x65, // "add_and_double"
	0x00, 0x02, // kind=func, index=2
	// export "multiply_result"
	0x10, 0x6d, 0x75, 0x6c, 0x74, 0x69, 0x70, 0x6c, 0x79, 0x5f, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, // "multiply_result"
	0x00, 0x03, // kind=func, index=3

	// Code section (id=10): 2 function bodies
	0x0a, 0x14, 0x02, // section id=10, size=20, 2 bodies

	// func body 2: add_and_double(a, b) -> a+b 然后乘2
	0x0c, 0x00, // body size=12, 0 locals
	0x20, 0x00, // local.get 0 (a)
	0x20, 0x01, // local.get 1 (b)
	0x10, 0x00, // call $custom_add (import 0)
	0x41, 0x02, // i32.const 2
	0x6c, // i32.mul
	0x0b, // end

	// func body 3: multiply_result(a, b) -> a*b
	0x06, 0x00, // body size=6, 0 locals
	0x20, 0x00, // local.get 0 (a)
	0x20, 0x01, // local.get 1 (b)
	0x10, 0x01, // call $custom_multiply (import 1)
	0x0b, // end
}

func main() {
	ctx := context.Background()

	// 创建自定义ABI
	customABI := abi.NewCustomABIBuilder("my_custom_abi", "1.0.0").
		WithModule("env").
		WithHostFunction("custom_add", func(ctx context.Context, module api.Module, a, b int32) int32 {
			return a + b
		}).
		WithHostFunction("custom_multiply", func(ctx context.Context, module api.Module, a, b int32) int32 {
			return a * b
		}).
		Build()

	// 创建执行器
	executor, err := engine.NewExecutor(ctx, &engine.ExecutorConfig{
		RuntimeConfig: &engine.RuntimeConfig{},
		ABIs:          []engine.ABI{customABI},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer executor.Close()

	fmt.Println("=== 自定义ABI执行器 ===")
	fmt.Printf("已注册ABI: %s v%s\n", customABI.Name(), customABI.Version())
	fmt.Printf("支持模块: %v\n", customABI.Modules())
	fmt.Println()

	// 加载测试模块
	if err := executor.LoadModule(ctx, "custom", testCustomWASM); err != nil {
		log.Fatalf("加载模块失败: %v", err)
	}
	fmt.Println("✅ 模块加载成功")

	// 执行 add_and_double(3, 4) = (3+4)*2 = 14
	result, err := executor.Execute("custom", "add_and_double", 3, 4)
	if err != nil {
		log.Fatalf("执行 add_and_double 失败: %v", err)
	}
	fmt.Printf("add_and_double(3, 4) = %d (预期: 14)\n", result.Values[0])

	// 执行 multiply_result(5, 6) = 5*6 = 30
	result, err = executor.Execute("custom", "multiply_result", 5, 6)
	if err != nil {
		log.Fatalf("执行 multiply_result 失败: %v", err)
	}
	fmt.Printf("multiply_result(5, 6) = %d (预期: 30)\n", result.Values[0])

	fmt.Println()
	fmt.Println("✅ 自定义ABI验证通过!")
}
