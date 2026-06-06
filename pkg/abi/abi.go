// pkg/abi/abi.go
// ABI接口定义和管理

package abi

import (
	"fmt"

	"wasm-executor/pkg/types"
)

// Manager ABI管理器
type Manager struct {
	abis map[string]types.ABI
	// 模块名到ABI的映射
	moduleMap map[string]types.ABI
}

// NewManager 创建新的ABI管理器
func NewManager() *Manager {
	return &Manager{
		abis:      make(map[string]types.ABI),
		moduleMap: make(map[string]types.ABI),
	}
}

// Register 注册ABI
func (m *Manager) Register(a types.ABI) error {
	name := a.Name()
	if _, exists := m.abis[name]; exists {
		return fmt.Errorf("ABI %s already registered", name)
	}

	m.abis[name] = a

	// 建立模块映射
	for _, module := range a.Modules() {
		m.moduleMap[module] = a
	}

	return nil
}

// Get 通过名称获取ABI
func (m *Manager) Get(name string) (types.ABI, bool) {
	a, ok := m.abis[name]
	return a, ok
}

// GetByModule 通过模块名获取ABI
func (m *Manager) GetByModule(moduleName string) (types.ABI, bool) {
	a, ok := m.moduleMap[moduleName]
	return a, ok
}

// List 列出所有已注册的ABI
func (m *Manager) List() []types.ABI {
	result := make([]types.ABI, 0, len(m.abis))
	for _, a := range m.abis {
		result = append(result, a)
	}
	return result
}

// Unregister 注销ABI
func (m *Manager) Unregister(name string) {
	if a, ok := m.abis[name]; ok {
		delete(m.abis, name)
		for _, module := range a.Modules() {
			delete(m.moduleMap, module)
		}
	}
}
