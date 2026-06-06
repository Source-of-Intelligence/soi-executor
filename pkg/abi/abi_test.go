// pkg/abi/abi_test.go
// ABI管理器测试

package abi

import (
	"testing"

	"wasm-executor/pkg/types"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("manager is nil")
	}
	if len(m.abis) != 0 {
		t.Fatal("new manager should have no abis")
	}
}

func TestManagerRegister(t *testing.T) {
	m := NewManager()

	// 创建测试ABI
	abi := NewCustomABI("test_abi", "1.0.0", []string{"env"})

	// 注册ABI
	err := m.Register(abi)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// 重复注册应该失败
	err = m.Register(abi)
	if err == nil {
		t.Fatal("Register duplicate should fail")
	}
}

func TestManagerGet(t *testing.T) {
	m := NewManager()

	abi := NewCustomABI("test_abi", "1.0.0", []string{"env"})
	m.Register(abi)

	// 通过名称获取
	found, ok := m.Get("test_abi")
	if !ok {
		t.Fatal("Get by name failed")
	}
	if found.Name() != "test_abi" {
		t.Fatal("wrong abi name")
	}

	// 获取不存在的ABI
	_, ok = m.Get("nonexistent")
	if ok {
		t.Fatal("Get nonexistent should fail")
	}
}

func TestManagerGetByModule(t *testing.T) {
	m := NewManager()

	abi := NewCustomABI("test_abi", "1.0.0", []string{"env", "custom"})
	m.Register(abi)

	// 通过模块名获取
	found, ok := m.GetByModule("env")
	if !ok {
		t.Fatal("GetByModule failed")
	}
	if found.Name() != "test_abi" {
		t.Fatal("wrong abi")
	}

	// 获取不存在的模块
	_, ok = m.GetByModule("nonexistent")
	if ok {
		t.Fatal("GetByModule nonexistent should fail")
	}
}

func TestManagerList(t *testing.T) {
	m := NewManager()

	abi1 := NewCustomABI("abi1", "1.0.0", []string{"env"})
	abi2 := NewCustomABI("abi2", "1.0.0", []string{"custom"})

	m.Register(abi1)
	m.Register(abi2)

	abis := m.List()
	if len(abis) != 2 {
		t.Fatalf("expected 2 abis, got %d", len(abis))
	}
}

func TestManagerUnregister(t *testing.T) {
	m := NewManager()

	abi := NewCustomABI("test_abi", "1.0.0", []string{"env"})
	m.Register(abi)

	// 注销
	m.Unregister("test_abi")

	// 确认已注销
	_, ok := m.Get("test_abi")
	if ok {
		t.Fatal("Unregister failed")
	}
}

func TestCustomABIBuilder(t *testing.T) {
	abi := NewCustomABIBuilder("builder_test", "1.0.0").
		WithModule("custom").
		WithHostFunction("test_fn", func() {}).
		Build()

	if abi.Name() != "builder_test" {
		t.Fatal("wrong name")
	}

	modules := abi.Modules()
	if len(modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(modules))
	}
}

func TestBlockchainABI(t *testing.T) {
	ctx := &BlockchainContext{
		BlockHeight: 100,
	}

	abi := NewBlockchainABI(ctx)

	if abi.Name() != "blockchain" {
		t.Fatal("wrong name")
	}

	if abi.Version() != "1.0.0" {
		t.Fatal("wrong version")
	}

	modules := abi.Modules()
	if len(modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(modules))
	}
}

// 验证 CustomABI 实现了 types.ABI 接口
var _ types.ABI = (*CustomABI)(nil)

// 验证 BlockchainABI 实现了 types.ABI 接口
var _ types.ABI = (*BlockchainABI)(nil)
