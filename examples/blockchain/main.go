// examples/blockchain/main.go
// 区块链智能合约执行示例 - 使用内嵌WASM字节码验证区块链ABI

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Source-of-Intelligence/soi-executor/pkg/abi"
	"github.com/Source-of-Intelligence/soi-executor/pkg/engine"
)

// SimpleStorage 简单存储实现
type SimpleStorage struct {
	data map[string][]byte
}

func NewSimpleStorage() *SimpleStorage {
	return &SimpleStorage{data: make(map[string][]byte)}
}

func (s *SimpleStorage) Get(key []byte) ([]byte, error) {
	return s.data[string(key)], nil
}

func (s *SimpleStorage) Set(key, value []byte) error {
	s.data[string(key)] = value
	return nil
}

func (s *SimpleStorage) Delete(key []byte) error {
	delete(s.data, string(key))
	return nil
}

// testContractWASM 是一个简单的WASM模块，导出 store_value 和 get_block_height_test 函数
// 它通过导入 "env" 模块调用区块链ABI的主机函数
//
// WAT (WebAssembly Text Format):
// (module
//
//	(import "env" "storage_write" (func $storage_write (param i32 i32 i32 i32) (result i32)))
//	(import "env" "get_block_height" (func $get_block_height (result i64)))
//	(import "env" "log" (func $log (param i32 i32)))
//	(memory (export "memory") 1)
//	(data (i32.const 0) "hello")    // offset 0: "hello" (5 bytes)
//	(data (i32.const 8) "world")    // offset 8: "world" (5 bytes)
//
//	(func (export "store_value") (result i32)
//	  ;; 调用 storage_write(key_ptr=0, key_len=5, val_ptr=8, val_len=5)
//	  (call $storage_write (i32.const 0) (i32.const 5) (i32.const 8) (i32.const 5))
//	)
//
//	(func (export "get_block_height_test") (result i64)
//	  (call $get_block_height)
//	)
//
// )
var testContractWASM = []byte{
	// WASM header
	0x00, 0x61, 0x73, 0x6d, // magic
	0x01, 0x00, 0x00, 0x00, // version 1

	// === Type section (id=1) ===
	0x01, 0x14, 0x03, // section id=1, size=20, 3 types
	// type 0: (i32, i32, i32, i32) -> i32  [storage_write]
	0x60, 0x04, 0x7f, 0x7f, 0x7f, 0x7f, 0x01, 0x7f,
	// type 1: () -> i64  [get_block_height]
	0x60, 0x00, 0x01, 0x7e,
	// type 2: (i32, i32) -> ()  [log]
	0x60, 0x02, 0x7f, 0x7f, 0x00,

	// === Import section (id=2) ===
	0x02, 0x2e, 0x03, // section id=2, size=46, 3 imports
	// import 0: "env"."storage_write" -> type 0
	0x03, 0x65, 0x6e, 0x76, // "env"
	0x0d, 0x73, 0x74, 0x6f, 0x72, 0x61, 0x67, 0x65, 0x5f, 0x77, 0x72, 0x69, 0x74, 0x65, // "storage_write"
	0x00, 0x00, // kind=func, type index=0
	// import 1: "env"."get_block_height" -> type 1
	0x03, 0x65, 0x6e, 0x76, // "env"
	0x10, 0x67, 0x65, 0x74, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x68, 0x65, 0x69, 0x67, 0x68, 0x74, // "get_block_height"
	0x00, 0x01, // kind=func, type index=1
	// import 2: "env"."log" -> type 2
	0x03, 0x65, 0x6e, 0x76, // "env"
	0x03, 0x6c, 0x6f, 0x67, // "log"
	0x00, 0x02, // kind=func, type index=2

	// === Function section (id=3) ===
	0x03, 0x04, 0x02, // section id=3, size=4, 2 functions
	0x01, // func 3: type 1 (-> i64)
	0x01, // func 4: type 1 (-> i64)

	// === Memory section (id=5) ===
	0x05, 0x03, 0x01, // section id=5, size=3, 1 memory
	0x00, 0x01, // limits: min=1 page

	// === Export section (id=7) ===
	0x07, 0x1f, 0x03, // section id=7, size=31, 3 exports
	// export "memory"
	0x06, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, // "memory"
	0x02, 0x00, // kind=memory, index=0
	// export "store_value"
	0x0b, 0x73, 0x74, 0x6f, 0x72, 0x65, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, // "store_value"
	0x00, 0x03, // kind=func, index=3
	// export "get_block_height_test"
	0x14, 0x67, 0x65, 0x74, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x5f, 0x74, 0x65, 0x73, 0x74, // "get_block_height_test"
	0x00, 0x04, // kind=func, index=4

	// === Data section (id=11) ===
	0x0b, 0x12, 0x01, // section id=11, size=18, 1 data segment
	0x00,             // active segment, memory index=0
	0x41, 0x00, 0x0b, // i32.const 0, end
	0x0d,                    // data count=13
	'h', 'e', 'l', 'l', 'o', // "hello" at offset 0
	0x00,                    // padding null byte at offset 5
	'w', 'o', 'r', 'l', 'd', // "world" at offset 6
	0x00, 0x00, // padding

	// === Code section (id=10) ===
	0x0a, 0x1e, 0x02, // section id=10, size=30, 2 function bodies

	// func body 3: store_value() -> i32
	0x0d, 0x00, // body size=13, 0 locals
	0x41, 0x00, // i32.const 0  (key_ptr)
	0x41, 0x05, // i32.const 5  (key_len)
	0x41, 0x06, // i32.const 6  (val_ptr, "world")
	0x41, 0x05, // i32.const 5  (val_len)
	0x10, 0x00, // call import 0 (storage_write)
	0x0b, // end

	// func body 4: get_block_height_test() -> i64
	0x09, 0x00, // body size=9, 0 locals
	0x10, 0x01, // call import 1 (get_block_height)
	0x0b, // end
}

func main() {
	ctx := context.Background()

	// 创建存储
	storage := NewSimpleStorage()

	// 创建区块链上下文
	blockchainCtx := &abi.BlockchainContext{
		Caller:          []byte("0x1234567890abcdef"),
		ContractAddress: []byte("0xfedcba0987654321"),
		BlockHeight:     12345,
		BlockTimestamp:  1678901234,
		TxHash:          []byte("0xabcdef1234567890"),
		GasLimit:        1000000,
		GasUsed:         0,
		Storage:         storage,
	}

	// 创建区块链ABI
	blockchainABI := abi.NewBlockchainABI(blockchainCtx)

	// 创建执行器
	executor, err := engine.NewExecutor(ctx, &engine.ExecutorConfig{
		RuntimeConfig: &engine.RuntimeConfig{},
		ABIs:          []engine.ABI{blockchainABI},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer executor.Close()

	fmt.Println("=== 区块链WASM执行器 ===")
	fmt.Printf("调用者: %x\n", blockchainCtx.Caller)
	fmt.Printf("合约地址: %x\n", blockchainCtx.ContractAddress)
	fmt.Printf("区块高度: %d\n", blockchainCtx.BlockHeight)
	fmt.Println()

	// 加载测试合约
	if err := executor.LoadModule(ctx, "contract", testContractWASM); err != nil {
		log.Fatalf("加载模块失败: %v", err)
	}
	fmt.Println("✅ 合约加载成功")

	// 执行 store_value：将 "hello" -> "world" 写入存储
	result, err := executor.Execute("contract", "store_value")
	if err != nil {
		log.Fatalf("执行 store_value 失败: %v", err)
	}
	fmt.Printf("store_value 返回值: %d (0=成功)\n", result.Values[0])

	// 验证存储
	val, _ := storage.Get([]byte("hello"))
	fmt.Printf("存储验证: key='hello' -> value='%s'\n", string(val))

	// 执行 get_block_height_test：获取区块高度
	result, err = executor.Execute("contract", "get_block_height_test")
	if err != nil {
		log.Fatalf("执行 get_block_height_test 失败: %v", err)
	}
	fmt.Printf("get_block_height_test 返回值: %d (预期: 12345)\n", result.Values[0])

	fmt.Println()
	fmt.Println("✅ 区块链ABI验证通过!")
}
