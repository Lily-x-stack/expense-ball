package tools

// add_record 工具：记录一笔收入或支出。
// 这里只保留给大模型看的工具定义（名字、描述、参数说明），
// 真正的业务逻辑在 service.go 的 AddRecord，通过 Fn 直接引用。
import (
	"github.com/openai/openai-go/v3"
)

func NewAddRecord() *ToolDef {
	return &ToolDef{
		Name:        "add_record",
		Description: "记录一笔收入或支出。当用户说花钱、买东西、收到工资、转账等场景时调用",
		Params: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"type": map[string]any{
					"type":        "string",
					"enum":        []string{"expense", "income"},
					"description": "expense 表示支出（花出去的钱），income 表示收入（收到的钱）",
				},
				"amount": map[string]string{
					"type":        "number",
					"description": "金额，必须是正数，例如 15.5",
				},
				"category": map[string]string{
					"type":        "string",
					"description": "分类。支出建议用：餐饮、交通、购物、娱乐、医疗、学习、居住、其他；收入建议用：工资、兼职、红包、理财、其他",
				},
				"note": map[string]string{
					"type":        "string",
					"description": "这笔钱的简短备注，例如 午饭、打车、兼职工资",
				},
				"date": map[string]string{
					"type":        "string",
					"description": "可选，日期，格式 YYYY-MM-DD。用户说今天就填今天，不填默认今天",
				},
			},
			"required": []string{"type", "amount", "category"},
		},
		Fn: AddRecord,
	}
}
