package tools

// query_records 工具：按天或按月查询账目，按分类汇总收入和支出。
// 这里只保留给大模型看的工具定义，真正的业务逻辑在 service.go 的 QueryRecords。
import (
	"github.com/openai/openai-go/v3"
)

func NewQueryRecords() *ToolDef {
	return &ToolDef{
		Name:        "query_records",
		Description: "查询账目记录，按分类汇总收入和支出。可以查某一天、某个月，或全部记录。用户问花了多少、收入多少、账目明细、账单时调用",
		Params: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"date": map[string]string{
					"type":        "string",
					"description": "可选，查某一天，格式 YYYY-MM-DD，例如 2026-10-14",
				},
				"month": map[string]string{
					"type":        "string",
					"description": "可选，查某个月，格式 YYYY-MM，例如 2026-10",
				},
			},
			"required": []string{},
		},
		Fn: QueryRecords,
	}
}
