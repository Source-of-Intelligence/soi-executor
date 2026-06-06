// pkg/abi/blockchain.go
// 区块链智能合约ABI实现

package abi

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/tetratelabs/wazero/api"
	"wasm-executor/pkg/types"
)

// BlockchainContext 区块链上下文
type BlockchainContext struct {
	// 调用者地址
	Caller []byte
	// 合约地址
	ContractAddress []byte
	// 当前区块高度
	BlockHeight uint64
	// 当前区块时间戳
	BlockTimestamp uint64
	// 交易哈希
	TxHash []byte
	//  Gas限制
	GasLimit uint64
	// 已使用的Gas
	GasUsed uint64
	// 存储接口
	Storage StorageInterface
}

// StorageInterface 存储接口
type StorageInterface interface {
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	Delete(key []byte) error
}

// BlockchainABI 区块链ABI实现
type BlockchainABI struct {
	ctx *BlockchainContext
}

// NewBlockchainABI 创建区块链ABI
func NewBlockchainABI(ctx *BlockchainContext) *BlockchainABI {
	if ctx == nil {
		ctx = &BlockchainContext{}
	}
	return &BlockchainABI{ctx: ctx}
}

// Name 返回ABI名称
func (b *BlockchainABI) Name() string {
	return "blockchain"
}

// Version 返回ABI版本
func (b *BlockchainABI) Version() string {
	return "1.0.0"
}

// Modules 返回此ABI处理的模块名称列表
func (b *BlockchainABI) Modules() []string {
	return []string{"env", "blockchain"}
}

// SetupHostFunctions 设置主机函数到运行时
func (b *BlockchainABI) SetupHostFunctions(ctx context.Context, runtime types.RuntimeInterface, moduleName string) error {
	// 批量注册所有区块链主机函数
	functions := map[string]interface{}{
		"storage_read":         b.storageRead,
		"storage_write":        b.storageWrite,
		"storage_delete":       b.storageDelete,
		"get_caller":           b.getCaller,
		"get_contract_address": b.getContractAddress,
		"get_block_height":     b.getBlockHeight,
		"get_block_timestamp":  b.getBlockTimestamp,
		"get_tx_hash":          b.getTxHash,
		"get_gas_limit":        b.getGasLimit,
		"get_gas_used":         b.getGasUsed,
		"log":                  b.log,
		"abort":                b.abort,
	}

	return runtime.RegisterHostFunctions(moduleName, functions)
}

// SetContext 设置区块链上下文
func (b *BlockchainABI) SetContext(ctx *BlockchainContext) {
	b.ctx = ctx
}

// 存储函数实现

func (b *BlockchainABI) storageRead(ctx context.Context, module api.Module, keyPtr, keyLen, valPtr, valLen, writtenPtr uint32) uint32 {
	if b.ctx == nil || b.ctx.Storage == nil {
		return 1 // 错误码
	}

	key := readMemory(module, keyPtr, keyLen)
	value, err := b.ctx.Storage.Get(key)
	if err != nil {
		return 1
	}

	if uint32(len(value)) > valLen {
		return 2 // 缓冲区不足
	}

	writeMemory(module, valPtr, value)
	writeU32(module, writtenPtr, uint32(len(value)))
	return 0
}

func (b *BlockchainABI) storageWrite(ctx context.Context, module api.Module, keyPtr, keyLen, valPtr, valLen uint32) uint32 {
	if b.ctx == nil || b.ctx.Storage == nil {
		return 1
	}

	key := readMemory(module, keyPtr, keyLen)
	value := readMemory(module, valPtr, valLen)

	if err := b.ctx.Storage.Set(key, value); err != nil {
		return 1
	}
	return 0
}

func (b *BlockchainABI) storageDelete(ctx context.Context, module api.Module, keyPtr, keyLen uint32) uint32 {
	if b.ctx == nil || b.ctx.Storage == nil {
		return 1
	}

	key := readMemory(module, keyPtr, keyLen)
	if err := b.ctx.Storage.Delete(key); err != nil {
		return 1
	}
	return 0
}

// 区块链信息函数实现

func (b *BlockchainABI) getCaller(ctx context.Context, module api.Module, bufPtr, bufLen uint32) uint32 {
	if b.ctx == nil {
		return 1
	}
	return writeBytesOrError(module, b.ctx.Caller, bufPtr, bufLen)
}

func (b *BlockchainABI) getContractAddress(ctx context.Context, module api.Module, bufPtr, bufLen uint32) uint32 {
	if b.ctx == nil {
		return 1
	}
	return writeBytesOrError(module, b.ctx.ContractAddress, bufPtr, bufLen)
}

func (b *BlockchainABI) getBlockHeight(ctx context.Context, module api.Module) uint64 {
	if b.ctx == nil {
		return 0
	}
	return b.ctx.BlockHeight
}

func (b *BlockchainABI) getBlockTimestamp(ctx context.Context, module api.Module) uint64 {
	if b.ctx == nil {
		return 0
	}
	return b.ctx.BlockTimestamp
}

func (b *BlockchainABI) getTxHash(ctx context.Context, module api.Module, bufPtr, bufLen uint32) uint32 {
	if b.ctx == nil {
		return 1
	}
	return writeBytesOrError(module, b.ctx.TxHash, bufPtr, bufLen)
}

func (b *BlockchainABI) getGasLimit(ctx context.Context, module api.Module) uint64 {
	if b.ctx == nil {
		return 0
	}
	return b.ctx.GasLimit
}

func (b *BlockchainABI) getGasUsed(ctx context.Context, module api.Module) uint64 {
	if b.ctx == nil {
		return 0
	}
	return b.ctx.GasUsed
}

// 工具函数

func (b *BlockchainABI) log(ctx context.Context, module api.Module, msgPtr, msgLen uint32) {
	msg := readMemory(module, msgPtr, msgLen)
	fmt.Printf("[WASM LOG] %s\n", string(msg))
}

func (b *BlockchainABI) abort(ctx context.Context, module api.Module, msgPtr, msgLen, filePtr, fileLen, line, col uint32) {
	msg := readMemory(module, msgPtr, msgLen)
	file := readMemory(module, filePtr, fileLen)
	fmt.Printf("[WASM ABORT] %s at %s:%d:%d\n", string(msg), string(file), line, col)
}

// 辅助函数

func readMemory(module api.Module, ptr, len uint32) []byte {
	mem, ok := module.Memory().Read(ptr, len)
	if !ok {
		return nil
	}
	// 复制数据避免底层内存变化
	result := make([]byte, len)
	copy(result, mem)
	return result
}

func writeMemory(module api.Module, ptr uint32, data []byte) {
	module.Memory().Write(ptr, data)
}

func writeU32(module api.Module, ptr, val uint32) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, val)
	module.Memory().Write(ptr, buf)
}

func writeBytesOrError(module api.Module, data []byte, bufPtr, bufLen uint32) uint32 {
	if uint32(len(data)) > bufLen {
		return 2 // 缓冲区不足
	}
	writeMemory(module, bufPtr, data)
	return 0
}
