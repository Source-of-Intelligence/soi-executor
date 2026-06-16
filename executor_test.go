package executor

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Source-of-Intelligence/soi-vos"
)

// ============================================================
// E-01: Executor 创建测试
// ============================================================
func TestE01_ExecutorCreation(t *testing.T) {
	exec := NewExecutor(nil)
	if exec == nil {
		t.Fatal("Executor should not be nil")
	}
}

// ============================================================
// E-02: 工具注册测试
// ============================================================
func TestE02_ToolRegistration(t *testing.T) {
	exec := &Executor{
		tools: make(map[string]ToolHandler),
	}

	exec.RegisterTool("test_tool", func(args json.RawMessage) (interface{}, error) {
		return "registered", nil
	})

	if len(exec.tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(exec.tools))
	}
}

// ============================================================
// E-03: 工具执行测试
// ============================================================
func TestE03_ToolExecution(t *testing.T) {
	exec := &Executor{
		tools: make(map[string]ToolHandler),
	}

	exec.RegisterTool("echo", func(args json.RawMessage) (interface{}, error) {
		var params struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, err
		}
		return params.Message, nil
	})

	result, err := exec.Execute("echo", json.RawMessage(`{"message": "hello"}`))
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result != "hello" {
		t.Errorf("Expected 'hello', got '%v'", result)
	}
}

// ============================================================
// E-04: 未知工具执行测试
// ============================================================
func TestE04_UnknownTool(t *testing.T) {
	exec := &Executor{
		tools: make(map[string]ToolHandler),
	}

	_, err := exec.Execute("non_existent", json.RawMessage(`{}`))
	if err == nil {
		t.Error("Expected error for unknown tool")
	}
}

// ============================================================
// E-05: 工具执行错误处理
// ============================================================
func TestE05_ToolError(t *testing.T) {
	exec := &Executor{
		tools: make(map[string]ToolHandler),
	}

	exec.RegisterTool("error_tool", func(args json.RawMessage) (interface{}, error) {
		return nil, errors.New("tool execution failed")
	})

	_, err := exec.Execute("error_tool", json.RawMessage(`{}`))
	if err == nil {
		t.Error("Expected error from tool")
	}

	if err.Error() != "tool execution failed" {
		t.Errorf("Expected 'tool execution failed', got '%s'", err.Error())
	}
}

// ============================================================
// E-06: VOS 接口测试
// ============================================================
func TestE06_VOSInterface(t *testing.T) {
	vos := &vos.VOS{}

	// Test VOS structure
	if vos == nil {
		t.Fatal("VOS should not be nil")
	}
}

// ============================================================
// E-07: 运行时创建测试
// ============================================================
func TestE07_RuntimeCreation(t *testing.T) {
	rt := NewRuntime()
	if rt == nil {
		t.Fatal("Runtime should not be nil")
	}

	if rt.timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", rt.timeout)
	}
}

// ============================================================
// E-08: 运行时超时配置
// ============================================================
func TestE08_RuntimeTimeout(t *testing.T) {
	rt := NewRuntime()
	rt.SetTimeout(60 * time.Second)

	if rt.timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", rt.timeout)
	}
}

// ============================================================
// E-09: 插件配置测试
// ============================================================
func TestE09_PluginConfig(t *testing.T) {
	config := PluginConfig{
		Name:        "test_plugin",
		Timeout:     30 * time.Second,
		MemoryLimit: 100 * 1024 * 1024, // 100MB
	}

	if config.Name != "test_plugin" {
		t.Error("Plugin name mismatch")
	}

	if config.Timeout != 30*time.Second {
		t.Error("Plugin timeout mismatch")
	}

	if config.MemoryLimit != 100*1024*1024 {
		t.Error("Plugin memory limit mismatch")
	}
}

// ============================================================
// E-10: Watcher 创建测试
// ============================================================
func TestE10_WatcherCreation(t *testing.T) {
	watcher := NewWatcher("/test/path")
	if watcher == nil {
		t.Fatal("Watcher should not be nil")
	}

	if watcher.path != "/test/path" {
		t.Errorf("Expected path '/test/path', got '%s'", watcher.path)
	}
}

// ============================================================
// E-11: 插件实例创建测试
// ============================================================
func TestE11_PluginInstance(t *testing.T) {
	instance := &PluginInstance{
		name:   "test",
		status: StatusIdle,
	}

	if instance.name != "test" {
		t.Error("Plugin instance name mismatch")
	}

	if instance.status != StatusIdle {
		t.Error("Plugin instance status mismatch")
	}
}

// ============================================================
// E-12: 插件状态转换测试
// ============================================================
func TestE12_PluginStatusTransition(t *testing.T) {
	instance := &PluginInstance{
		name:   "test",
		status: StatusIdle,
	}

	// Idle -> Running
	instance.status = StatusRunning
	if instance.status != StatusRunning {
		t.Error("Failed to transition to Running")
	}

	// Running -> Completed
	instance.status = StatusCompleted
	if instance.status != StatusCompleted {
		t.Error("Failed to transition to Completed")
	}

	// Completed -> Idle (reset)
	instance.status = StatusIdle
	if instance.status != StatusIdle {
		t.Error("Failed to reset to Idle")
	}
}

// ============================================================
// E-13: 执行结果序列化测试
// ============================================================
func TestE13_ExecutionResultSerialization(t *testing.T) {
	result := ExecutionResult{
		Success: true,
		Output:  []byte(`{"status": "ok"}`),
		Error:   "",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed ExecutionResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed.Success != result.Success {
		t.Error("Success field mismatch")
	}

	if string(parsed.Output) != string(result.Output) {
		t.Error("Output field mismatch")
	}
}

// ============================================================
// E-14: 参数解析测试
// ============================================================
func TestE14_ArgumentParsing(t *testing.T) {
	exec := &Executor{
		tools: make(map[string]ToolHandler),
	}

	exec.RegisterTool("add", func(args json.RawMessage) (interface{}, error) {
		var params struct {
			A int `json:"a"`
			B int `json:"b"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, err
		}
		return params.A + params.B, nil
	})

	result, err := exec.Execute("add", json.RawMessage(`{"a": 5, "b": 3}`))
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result != 8 {
		t.Errorf("Expected 8, got %v", result)
	}
}

// ============================================================
// E-15: 复杂参数测试
// ============================================================
func TestE15_ComplexArguments(t *testing.T) {
	exec := &Executor{
		tools: make(map[string]ToolHandler),
	}

	exec.RegisterTool("complex", func(args json.RawMessage) (interface{}, error) {
		var params struct {
			Name   string                 `json:"name"`
			Values []int                  `json:"values"`
			Nested map[string]interface{} `json:"nested"`
			Flag   bool                   `json:"flag"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"name":   params.Name,
			"count":  len(params.Values),
			"nested": params.Nested,
			"flag":   params.Flag,
		}, nil
	})

	args := json.RawMessage(`{
		"name": "test",
		"values": [1, 2, 3],
		"nested": {"key": "value"},
		"flag": true
	}`)

	result, err := exec.Execute("complex", args)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Result should be a map")
	}

	if resultMap["name"] != "test" {
		t.Errorf("Expected name 'test', got %v", resultMap["name"])
	}

	if resultMap["count"].(int) != 3 {
		t.Errorf("Expected count 3, got %v", resultMap["count"])
	}

	if resultMap["flag"] != true {
		t.Error("Expected flag true")
	}
}

// ============================================================
// E-16: 执行时间记录测试
// ============================================================
func TestE16_ExecutionTiming(t *testing.T) {
	result := &ExecutionResult{
		Success:   true,
		StartTime: time.Now(),
		Output:    []byte(`{}`),
	}

	// Simulate some work
	time.Sleep(10 * time.Millisecond)

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if result.Duration < 10*time.Millisecond {
		t.Error("Duration should be at least 10ms")
	}
}

// ============================================================
// E-17: ABI 版本检查
// ============================================================
func TestE17_ABIVersion(t *testing.T) {
	expectedVersion := soi_vos.ABIVersion

	if expectedVersion == "" {
		t.Error("ABI version should not be empty")
	}

	// Verify it's a valid format (e.g., "1.0" or "2.0")
	if len(expectedVersion) < 3 {
		t.Error("ABI version seems too short")
	}
}

// ============================================================
// E-18: 工具列表获取测试
// ============================================================
func TestE18_GetToolList(t *testing.T) {
	exec := &Executor{
		tools: make(map[string]ToolHandler),
	}

	exec.RegisterTool("tool1", nil)
	exec.RegisterTool("tool2", nil)
	exec.RegisterTool("tool3", nil)

	toolList := exec.GetToolList()
	if len(toolList) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(toolList))
	}
}

// ============================================================
// E-19: 插件重置测试
// ============================================================
func TestE19_PluginReset(t *testing.T) {
	exec := &Executor{
		tools: make(map[string]ToolHandler),
	}

	exec.RegisterTool("tool1", nil)

	exec.Reset()

	if len(exec.tools) != 0 {
		t.Errorf("Expected 0 tools after reset, got %d", len(exec.tools))
	}
}

// ============================================================
// E-20: 执行上下文测试
// ============================================================
func TestE20_ExecutionContext(t *testing.T) {
	ctx := &ExecutionContext{
		PluginName: "test_plugin",
		ToolName:   "test_tool",
		StartTime:  time.Now(),
		Timeout:    30 * time.Second,
	}

	if ctx.PluginName != "test_plugin" {
		t.Error("PluginName mismatch")
	}

	if ctx.ToolName != "test_tool" {
		t.Error("ToolName mismatch")
	}

	if ctx.Timeout != 30*time.Second {
		t.Error("Timeout mismatch")
	}

	// Check if context is expired
	if ctx.IsExpired() {
		t.Error("Fresh context should not be expired")
	}
}
