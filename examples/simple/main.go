// examples/simple/main.go
// 简单WASM执行示例

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Source-of-Intelligence/soi-executor/pkg/engine"
)

// 简单的WASM字节码 (add 函数: 返回两个参数的和)
var testWasmAdd = []byte{
	0x00, 0x61, 0x73, 0x6d, // WASM magic
	0x01, 0x00, 0x00, 0x00, // version 1
	// Type section
	0x01, 0x07, 0x01, // section id=1, size=7, 1 type
	0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f, // func type: (i32, i32) -> i32
	// Function section
	0x03, 0x02, 0x01, 0x00, // section id=3, size=2, 1 func, type 0
	// Export section
	0x07, 0x07, 0x01, // section id=7, size=7, 1 export
	0x03, 0x61, 0x64, 0x64, // name "add"
	0x00, 0x00, // kind func, index 0
	// Code section
	0x0a, 0x09, 0x01, // section id=10, size=9, 1 body
	0x07, 0x00, // body size=7, 0 locals
	0x20, 0x00, // local.get 0
	0x20, 0x01, // local.get 1
	0x6a, // i32.add
	0x0b, // end
}

func main() {
	ctx := context.Background()

	// 创建执行器
	executor, err := engine.NewExecutor(ctx, &engine.ExecutorConfig{
		RuntimeConfig: &engine.RuntimeConfig{},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer executor.Close()

	// 加载模块
	if err := executor.LoadModule(ctx, "math", testWasmAdd); err != nil {
		log.Fatal(err)
	}

	// 执行 add 函数 (1 + 2)
	result, err := executor.Execute("math", "add", 1, 2)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("1 + 2 = %d\n", result.Values[0])
}
