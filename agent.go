package main

// Agent 负责和大模型对话、识别用户意图，并通过 MCP Client 调用记账工具。
// 关键变化：工具不再写死在本地，而是启动时从 MCP Server 动态发现（ListTools），
// 模型要执行工具时，通过 MCP 协议的 CallTool 让 Server 去干活。
// 整体仍是一个循环：
//   用户说话 → 问大模型 → 大模型说要调工具 → 通过 MCP 执行 → 结果给大模型 → ……
//   直到大模型不再调工具，给出最终回答。
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"expense-ball/core"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

const MaxLoop = 10 // 最多循环多少轮，防止无限调用工具

type Agent struct {
	client      openai.Client
	session     *core.Session
	mcpClient   *mcpclient.Client
	openaiTools []responses.ToolUnionParam
}

// NewAgent 初始化大模型客户端，并连接 MCP Server、动态发现工具
func NewAgent() (*Agent, error) {
	session := core.NewSession()

	// 系统提示词：告诉大模型它是记账助手，以及分类规则
	session.AddMessage(
		responses.ResponseInputItemParamOfMessage(
			"你是一个记账助手。用户会用自然语言告诉你收入或支出，请调用 add_record 工具记录；"+
				"用户想查询账单时，请调用 query_records 工具，并根据工具返回的真实数据用中文简洁回答。"+
				"支出分类建议：餐饮、交通、购物、娱乐、医疗、学习、居住、其他；"+
				"收入分类建议：工资、兼职、红包、理财、其他。"+
				"回答要简洁、口语化，重点告诉用户记录了什么、花了多少、还剩多少。",
			responses.EasyInputMessageRoleSystem,
		),
	)

	// 1. 连接 MCP Server（stdio 本地子进程）。
	//    命令可用环境变量 MCP_SERVER_CMD 指定，默认找项目根目录下的 expense-mcp-server。
	serverCmd := os.Getenv("MCP_SERVER_CMD")
	if serverCmd == "" {
		// Go 出于安全不执行当前目录下的裸文件名，必须带显式相对路径
		serverCmd = "./expense-mcp-server"
		if runtime.GOOS == "windows" {
			serverCmd = "./expense-mcp-server.exe"
		}
	}
	mcpCli, err := mcpclient.NewStdioMCPClient(serverCmd, nil)
	if err != nil {
		return nil, fmt.Errorf("无法启动 MCP Server（命令 %q）：%w；请先用 go build 编译 MCP Server，或用环境变量 MCP_SERVER_CMD 指定其路径", serverCmd, err)
	}

	// 2. 握手并发现工具（给启动阶段一个独立超时）
	initCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := mcpCli.Initialize(initCtx, mcpsdk.InitializeRequest{}); err != nil {
		_ = mcpCli.Close()
		return nil, fmt.Errorf("MCP Initialize 失败：%w", err)
	}

	listResult, err := mcpCli.ListTools(initCtx, mcpsdk.ListToolsRequest{})
	if err != nil {
		_ = mcpCli.Close()
		return nil, fmt.Errorf("MCP ListTools 失败：%w", err)
	}

	// 3. 把 MCP 工具的参数说明转成 openai 需要的工具格式
	openaiTools := make([]responses.ToolUnionParam, 0, len(listResult.Tools))
	for _, t := range listResult.Tools {
		// InputSchema 是结构体，先转成 map[string]any 再交给 openai.FunctionParameters
		raw, err := json.Marshal(t.InputSchema)
		if err != nil {
			_ = mcpCli.Close()
			return nil, err
		}
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			_ = mcpCli.Close()
			return nil, err
		}
		openaiTools = append(openaiTools, responses.ToolUnionParam{
			OfFunction: &responses.FunctionToolParam{
				Name:        t.Name,
				Description: openai.String(t.Description),
				Parameters:  openai.FunctionParameters(schema),
			},
		})
	}

	// 4. 大模型 API Key：从环境变量 ARK_API_KEY 读取
	apiKey := os.Getenv("ARK_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("未设置环境变量 ARK_API_KEY，请在运行前配置")
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL("https://ark.cn-beijing.volces.com/api/v3"),
		option.WithRequestTimeout(30*time.Second),
	)

	return &Agent{
		client:      client,
		session:     session,
		mcpClient:   mcpCli,
		openaiTools: openaiTools,
	}, nil
}

// Close 关闭 MCP 连接、结束 server 子进程
func (a *Agent) Close() error {
	if a.mcpClient != nil {
		return a.mcpClient.Close()
	}
	return nil
}

// Run 处理用户的一句话，返回大模型的最终回答。
// logf 是过程日志回调，思考内容和工具调用会通过它显示在界面上。
func (a *Agent) Run(ctx context.Context, userInput string, logf func(string)) (string, error) {
	// 每轮对话都告诉大模型今天的日期，方便它理解"今天/昨天"
	a.session.AddMessage(
		responses.ResponseInputItemParamOfMessage(
			fmt.Sprintf("（当前日期：%s）%s", time.Now().Format("2006-01-02"), userInput),
			responses.EasyInputMessageRoleUser,
		),
	)

	for loop := 0; loop <= MaxLoop; loop++ {
		resp, err := a.client.Responses.New(ctx, responses.ResponseNewParams{
			Input: responses.ResponseNewParamsInputUnion{
				OfInputItemList: a.session.GetMessages(),
			},
			Tools: a.openaiTools,
			Model: "deepseek-v4-flash-ga-260731", // 火山方舟模型广场的 DeepSeek-V4-Flash 正式版 ID
		})
		if err != nil {
			// 打印 API 返回的详细错误，方便排查
			var apiErr *openai.Error
			if errors.As(err, &apiErr) {
				return "", fmt.Errorf("HTTP %d: %s", apiErr.StatusCode, apiErr.RawJSON())
			}
			return "", err
		}

		hasToolCall := false
		for _, item := range resp.Output {
			switch item.Type {
			case "reasoning":
				// 大模型的思考过程（有些模型会返回）
				for _, summary := range item.Summary {
					if summary.Text != "" {
						logf("思考：" + summary.Text)
					}
				}
			case "function_call":
				hasToolCall = true
				fc := item.AsFunctionCall()
				logf("调用工具：" + fc.Name + "，参数：" + fc.Arguments)

				// 把"工具调用"这件事加入会话
				a.session.AddMessage(responses.ResponseInputItemUnionParam{
					OfFunctionCall: &responses.ResponseFunctionToolCallParam{
						Arguments: fc.Arguments,
						CallID:    fc.CallID,
						Name:      fc.Name,
					},
				})

				// 通过 MCP 让 Server 执行工具（替换原来的本地 map 分发）
				toolResult := a.callMCPTool(ctx, fc.Name, fc.Arguments)
				logf("工具返回：" + toolResult)

				// 把工具返回结果加入会话，供大模型下一轮参考
				a.session.AddMessage(responses.ResponseInputItemUnionParam{
					OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
						Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
							OfString: openai.String(toolResult),
						},
						CallID: openai.String(fc.CallID),
					},
				})
			}
		}

		// 没有工具调用，说明大模型已经给出最终回答
		if !hasToolCall {
			reply := resp.OutputText()
			// 把助手的回答也加入会话，这样下一轮能记住上下文
			a.session.AddMessage(
				responses.ResponseInputItemParamOfMessage(reply, responses.EasyInputMessageRoleAssistant),
			)
			return reply, nil
		}
	}

	return "", fmt.Errorf("超过最大循环次数 %d，仍未得到最终回答", MaxLoop)
}

// callMCPTool 通过 MCP Client 调用指定工具，并把结果整理成文本
func (a *Agent) callMCPTool(ctx context.Context, name, argsJSON string) string {
	// 模型给的参数是 JSON 字符串，先解析成 map
	var args map[string]any
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return name + " 参数解析失败：" + err.Error()
		}
	}

	result, err := a.mcpClient.CallTool(ctx, mcpsdk.CallToolRequest{
		Params: mcpsdk.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	})
	if err != nil {
		// 协议层错误（比如连接断开），作为观察结果还给模型
		return fmt.Sprintf("%s 调用失败：%v", name, err)
	}

	text := mcpText(result)
	if text == "" {
		return name + " 没有返回文本内容"
	}
	return text
}

// mcpText 提取 CallToolResult 里所有文本类型的内容
func mcpText(result *mcpsdk.CallToolResult) string {
	var sb strings.Builder
	for _, c := range result.Content {
		// MCP 内容是接口，文本内容对应 TextContent 结构体
		if tc, ok := c.(mcpsdk.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}
