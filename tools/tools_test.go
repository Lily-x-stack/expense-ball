package tools

// 工具层的简单单元测试（不依赖大模型，直接测记账和查询逻辑）
// 运行：在项目根目录执行 go test ./...
import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// setupTest 测试前清空数据文件，测试结束后恢复
func setupTest(t *testing.T) {
	t.Helper()
	os.Remove(dataFile)
	t.Cleanup(func() { os.Remove(dataFile) })
}

// 用工具自己的 Call 入口（参数是 JSON 字符串），模拟大模型调用
func callTool(t *testing.T, tool *ToolDef, args map[string]any) string {
	t.Helper()
	raw, _ := json.Marshal(args)
	result, err := tool.Call(context.Background(), string(raw))
	if err != nil {
		t.Fatalf("工具执行失败: %v", err)
	}
	return result
}

func TestAddAndQuery(t *testing.T) {
	setupTest(t)
	ctx := context.Background()
	_ = ctx

	add := NewAddRecord()
	query := NewQueryRecords()
	today := time.Now().Format("2006-01-02")

	// 1. 记一笔支出：午饭 15
	r1 := callTool(t, add, map[string]any{
		"type": "expense", "amount": 15.0, "category": "餐饮", "note": "午饭",
	})
	if !strings.Contains(r1, "已记录") {
		t.Fatalf("记账失败，返回：%s", r1)
	}

	// 2. 再记一笔支出：打车 22.5
	callTool(t, add, map[string]any{
		"type": "expense", "amount": 22.5, "category": "交通", "note": "打车",
	})

	// 3. 记一笔收入：兼职 500
	callTool(t, add, map[string]any{
		"type": "income", "amount": 500.0, "category": "兼职", "note": "周末兼职",
	})

	// 4. 查今天的账
	report := callTool(t, query, map[string]any{"date": today})
	t.Log("\n" + report)

	wantContains := []string{"餐饮: 15.00", "交通: 22.50", "兼职: 500.00",
		"支出合计: 37.50", "收入合计: 500.00", "结余（收入-支出）: 462.50"}
	for _, want := range wantContains {
		if !strings.Contains(report, want) {
			t.Errorf("查询结果里缺少 %q", want)
		}
	}

	// 5. 确认文件里确实存了 3 条
	records, err := loadRecords()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Errorf("期望 3 条记录，实际 %d 条", len(records))
	}
}

func TestQueryEmpty(t *testing.T) {
	setupTest(t)
	query := NewQueryRecords()
	// 没有任何记录时应该友好提示，而不是报错
	r := callTool(t, query, map[string]any{"month": "2026-09"})
	if !strings.Contains(r, "没有账目记录") {
		t.Fatalf("空数据提示不正确：%s", r)
	}
}
