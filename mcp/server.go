package mcp

// MCP Server：把记账、查账两个能力通过 MCP 协议暴露出去。
// 业务逻辑直接复用 tools 包的 AddRecord / QueryRecords，不重复实现。
import (
	"context"

	"expense-ball/tools"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewServer 构建一个已经注册好记账工具的 MCP Server
func NewServer() *server.MCPServer {
	s := server.NewMCPServer(
		"expense-ball-mcp",
		"1.0.0",
		server.WithRecovery(), // handler 发生 panic 时自动恢复，不会让整个 server 崩掉
	)

	// 工具 1：add_record
	addTool := mcpsdk.NewTool("add_record",
		mcpsdk.WithDescription("记录一笔收入或支出。当用户说花钱、买东西、收到工资、转账等场景时调用"),
		mcpsdk.WithString("type",
			mcpsdk.Required(),
			mcpsdk.Description("expense 表示支出（花出去的钱），income 表示收入（收到的钱）"),
			mcpsdk.Enum("expense", "income"),
		),
		mcpsdk.WithNumber("amount",
			mcpsdk.Required(),
			mcpsdk.Description("金额，必须是正数，例如 15.5"),
		),
		mcpsdk.WithString("category",
			mcpsdk.Required(),
			mcpsdk.Description("分类。支出：餐饮、交通、购物、娱乐、医疗、学习、居住、其他；收入：工资、兼职、红包、理财、其他"),
		),
		mcpsdk.WithString("note",
			mcpsdk.Description("这笔钱的简短备注，例如 午饭、打车、兼职工资"),
		),
		mcpsdk.WithString("date",
			mcpsdk.Description("可选，日期 YYYY-MM-DD，不填默认今天"),
		),
	)
	s.AddTool(addTool, func(ctx context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		out, err := tools.AddRecord(ctx, req.GetArguments())
		if err != nil {
			// 业务错误放进 result 并标记 IsError，让大模型能看到错误并自我纠正
			return mcpsdk.NewToolResultError(err.Error()), nil
		}
		return mcpsdk.NewToolResultText(out), nil
	})

	// 工具 2：query_records
	queryTool := mcpsdk.NewTool("query_records",
		mcpsdk.WithDescription("查询账目记录，按分类汇总收入和支出。可查某一天、某个月或全部。用户问花了多少、收入、明细、账单时调用"),
		mcpsdk.WithString("date",
			mcpsdk.Description("可选，查某一天，格式 YYYY-MM-DD，例如 2026-10-14"),
		),
		mcpsdk.WithString("month",
			mcpsdk.Description("可选，查某个月，格式 YYYY-MM，例如 2026-10"),
		),
	)
	s.AddTool(queryTool, func(ctx context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		out, err := tools.QueryRecords(ctx, req.GetArguments())
		if err != nil {
			return mcpsdk.NewToolResultError(err.Error()), nil
		}
		return mcpsdk.NewToolResultText(out), nil
	})

	return s
}
