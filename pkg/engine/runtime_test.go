// pkg/engine/runtime_test.go
// 核心引擎测试

package engine

import (
	"context"
	"testing"
)

// 简单的WASM字节码 (add 函数: 返回两个参数的和)
// 使用wat2wasm工具生成的简单模块
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

func TestNewRuntime(t *testing.T) {
	ctx := context.Background()
	rt, err := NewRuntime(ctx, nil)
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}
	defer rt.Close()

	if rt == nil {
		t.Fatal("runtime is nil")
	}
}

func TestCompileAndExecute(t *testing.T) {
	ctx := context.Background()
	rt, err := NewRuntime(ctx, nil)
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}
	defer rt.Close()

	// 编译模块
	_, err = rt.CompileModule("math", testWasmAdd)
	if err != nil {
		t.Fatalf("CompileModule failed: %v", err)
	}

	// 实例化模块
	_, err = rt.InstantiateModule("math", nil)
	if err != nil {
		t.Fatalf("InstantiateModule failed: %v", err)
	}

	// 执行函数
	result, err := rt.ExecuteFunction("math", "add", 10, 20)
	if err != nil {
		t.Fatalf("ExecuteFunction failed: %v", err)
	}

	if len(result.Values) != 1 {
		t.Fatalf("expected 1 return value, got %d", len(result.Values))
	}

	if result.Values[0] != 30 {
		t.Fatalf("expected 30, got %d", result.Values[0])
	}
}

func TestRuntimeWithWASI(t *testing.T) {
	ctx := context.Background()
	config := &RuntimeConfig{
		EnableWASI: true,
	}

	rt, err := NewRuntime(ctx, config)
	if err != nil {
		t.Fatalf("NewRuntime with WASI failed: %v", err)
	}
	defer rt.Close()

	if rt.config.EnableWASI != true {
		t.Fatal("WASI should be enabled")
	}
}

func TestRegisterHostFunction(t *testing.T) {
	ctx := context.Background()
	rt, err := NewRuntime(ctx, nil)
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}
	defer rt.Close()

	// 注册主机函数
	hostFn := func(ctx context.Context, x int32) int32 {
		return x * 2
	}

	err = rt.RegisterHostFunction("env", "host_double", hostFn)
	if err != nil {
		t.Fatalf("RegisterHostFunction failed: %v", err)
	}
}
