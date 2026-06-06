// cmd/executor/main.go
// WASM执行器CLI工具

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"wasm-executor/pkg/abi"
	"wasm-executor/pkg/engine"
)

func main() {
	var (
		wasmFile         = flag.String("wasm", "", "WASM文件路径 (必填)")
		funcName         = flag.String("func", "", "要执行的函数名 (必填)")
		moduleName       = flag.String("module", "main", "模块名称")
		enableWASI       = flag.Bool("wasi", false, "启用WASI支持")
		enableBlockchain = flag.Bool("blockchain", false, "启用区块链ABI")
		params           = flag.String("params", "", "函数参数 (逗号分隔的整数)")
	)
	flag.Parse()

	if *wasmFile == "" || *funcName == "" {
		fmt.Println("用法: executor -wasm <file> -func <name> [-module <name>] [-wasi] [-blockchain]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// 读取WASM文件
	wasmBytes, err := os.ReadFile(*wasmFile)
	if err != nil {
		fmt.Printf("读取WASM文件失败: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// 创建执行器配置
	config := &engine.ExecutorConfig{
		RuntimeConfig: &engine.RuntimeConfig{
			EnableWASI: *enableWASI,
		},
	}

	// 如果启用区块链ABI
	if *enableBlockchain {
		blockchainABI := abi.NewBlockchainABI(&abi.BlockchainContext{
			Caller:          []byte("0x1234..."),
			ContractAddress: []byte("0x5678..."),
			BlockHeight:     100,
			BlockTimestamp:  1234567890,
			GasLimit:        1000000,
		})
		config.ABIs = append(config.ABIs, blockchainABI)
	}

	// 创建执行器
	executor, err := engine.NewExecutor(ctx, config)
	if err != nil {
		fmt.Printf("创建执行器失败: %v\n", err)
		os.Exit(1)
	}
	defer executor.Close()

	// 加载模块
	if err := executor.LoadModule(ctx, *moduleName, wasmBytes); err != nil {
		fmt.Printf("加载模块失败: %v\n", err)
		os.Exit(1)
	}

	// 解析参数
	var args []uint64
	if *params != "" {
		// 简单解析逗号分隔的整数
		var val uint64
		for _, p := range *params {
			if p == ',' {
				args = append(args, val)
				val = 0
				continue
			}
			if p >= '0' && p <= '9' {
				val = val*10 + uint64(p-'0')
			}
		}
		args = append(args, val)
	}

	// 执行函数
	fmt.Printf("执行 %s.%s(%v)...\n", *moduleName, *funcName, args)
	result, err := executor.Execute(*moduleName, *funcName, args...)
	if err != nil {
		fmt.Printf("执行失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ 执行成功!\n")
	fmt.Printf("返回值: %v\n", result.Values)
}
