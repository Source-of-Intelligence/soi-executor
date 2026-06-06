// pkg/context/injector.go
// Context注入机制，支持将Go context传递到WASM模块

package context

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Source-of-Intelligence/soi-executor/pkg/types"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// ContextData 可注入WASM的上下文数据
type ContextData struct {
	// 请求ID
	RequestID string `json:"request_id,omitempty"`
	// 用户ID
	UserID string `json:"user_id,omitempty"`
	// 时间戳
	Timestamp int64 `json:"timestamp,omitempty"`
	// 自定义数据
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// Injector Context注入器
type Injector struct {
	// 全局上下文数据
	globalData *ContextData
	// 上下文提供者函数
	provider func(context.Context) *ContextData
	// 确保只注册一次
	registerOnce sync.Once
	// 注册错误
	registerErr error
}

// NewInjector 创建新的Context注入器
func NewInjector() *Injector {
	return &Injector{
		globalData: &ContextData{
			Custom: make(map[string]interface{}),
		},
	}
}

// SetGlobalData 设置全局上下文数据
func (i *Injector) SetGlobalData(data *ContextData) {
	i.globalData = data
	if i.globalData.Custom == nil {
		i.globalData.Custom = make(map[string]interface{})
	}
}

// SetProvider 设置上下文提供者函数
func (i *Injector) SetProvider(provider func(context.Context) *ContextData) {
	i.provider = provider
}

// ApplyToModule 将Context注入到模块配置
func (i *Injector) ApplyToModule(ctx context.Context, runtime types.RuntimeInterface, config wazero.ModuleConfig) error {
	// 注册context相关的主机函数（只执行一次）
	i.registerOnce.Do(func() {
		i.registerErr = i.registerContextFunctions(ctx, runtime)
	})

	if i.registerErr != nil {
		return fmt.Errorf("failed to register context functions: %w", i.registerErr)
	}

	return nil
}

// registerContextFunctions 注册Context相关的主机函数
func (i *Injector) registerContextFunctions(ctx context.Context, runtime types.RuntimeInterface) error {
	// 注册所有context函数（使用批量注册）
	functions := map[string]interface{}{
		"ctx_get_data":       i.getContextData,
		"ctx_get_field":      i.getContextField,
		"ctx_get_request_id": i.getRequestID,
		"ctx_get_user_id":    i.getUserID,
		"ctx_has_field":      i.hasField,
	}

	return runtime.RegisterHostFunctions("context", functions)
}

// getCurrentData 获取当前上下文数据
func (i *Injector) getCurrentData(ctx context.Context) *ContextData {
	if i.provider != nil {
		return i.provider(ctx)
	}
	return i.globalData
}

// getContextData 获取完整的context数据JSON
func (i *Injector) getContextData(ctx context.Context, module api.Module, bufPtr, bufLen, writtenPtr uint32) uint32 {
	data := i.getCurrentData(ctx)
	if data == nil {
		return 1
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return 1
	}

	if uint32(len(jsonData)) > bufLen {
		return 2 // 缓冲区不足
	}

	module.Memory().Write(bufPtr, jsonData)

	// 写入实际长度
	lenBuf := []byte{
		byte(len(jsonData)),
		byte(len(jsonData) >> 8),
		byte(len(jsonData) >> 16),
		byte(len(jsonData) >> 24),
	}
	module.Memory().Write(writtenPtr, lenBuf)

	return 0
}

// getContextField 获取特定字段值
func (i *Injector) getContextField(ctx context.Context, module api.Module, fieldPtr, fieldLen, bufPtr, bufLen, writtenPtr uint32) uint32 {
	data := i.getCurrentData(ctx)
	if data == nil {
		return 1
	}

	// 读取字段名
	fieldBytes, ok := module.Memory().Read(fieldPtr, fieldLen)
	if !ok {
		return 1
	}
	fieldName := string(fieldBytes)

	var value interface{}
	switch fieldName {
	case "request_id":
		value = data.RequestID
	case "user_id":
		value = data.UserID
	case "timestamp":
		value = data.Timestamp
	default:
		value = data.Custom[fieldName]
	}

	if value == nil {
		return 3 // 字段不存在
	}

	jsonData, err := json.Marshal(value)
	if err != nil {
		return 1
	}

	if uint32(len(jsonData)) > bufLen {
		return 2
	}

	module.Memory().Write(bufPtr, jsonData)

	lenBuf := []byte{
		byte(len(jsonData)),
		byte(len(jsonData) >> 8),
		byte(len(jsonData) >> 16),
		byte(len(jsonData) >> 24),
	}
	module.Memory().Write(writtenPtr, lenBuf)

	return 0
}

// getRequestID 获取请求ID
func (i *Injector) getRequestID(ctx context.Context, module api.Module, bufPtr, bufLen uint32) uint32 {
	data := i.getCurrentData(ctx)
	if data == nil {
		return 1
	}

	reqID := []byte(data.RequestID)
	if uint32(len(reqID)) > bufLen {
		return 2
	}

	module.Memory().Write(bufPtr, reqID)
	return 0
}

// getUserID 获取用户ID
func (i *Injector) getUserID(ctx context.Context, module api.Module, bufPtr, bufLen uint32) uint32 {
	data := i.getCurrentData(ctx)
	if data == nil {
		return 1
	}

	userID := []byte(data.UserID)
	if uint32(len(userID)) > bufLen {
		return 2
	}

	module.Memory().Write(bufPtr, userID)
	return 0
}

// hasField 检查字段是否存在
func (i *Injector) hasField(ctx context.Context, module api.Module, fieldPtr, fieldLen uint32) uint32 {
	data := i.getCurrentData(ctx)
	if data == nil {
		return 0
	}

	fieldBytes, ok := module.Memory().Read(fieldPtr, fieldLen)
	if !ok {
		return 0
	}
	fieldName := string(fieldBytes)

	switch fieldName {
	case "request_id":
		if data.RequestID != "" {
			return 1
		}
	case "user_id":
		if data.UserID != "" {
			return 1
		}
	case "timestamp":
		if data.Timestamp != 0 {
			return 1
		}
	default:
		if _, exists := data.Custom[fieldName]; exists {
			return 1
		}
	}

	return 0
}

// WithValue 向全局context添加自定义值
func (i *Injector) WithValue(key string, value interface{}) {
	if i.globalData == nil {
		i.globalData = &ContextData{Custom: make(map[string]interface{})}
	}
	i.globalData.Custom[key] = value
}
