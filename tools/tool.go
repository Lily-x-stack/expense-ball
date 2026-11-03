package tools

// 每个工具都用 ToolDef 来描述：
// 大模型看到的是 Name / Description / Params（说明书），
// 真正执行时调用的是 Fn（Go 函数）。
import (
	"context"
	"encoding/json"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
)

type ToolDef struct {
	Name        string                                                         // 工具名字
	Description string                                                         // 工具是干什么的（给大模型看）
	Params      openai.FunctionParameters                                      // 工具参数说明（JSON Schema）
	Fn          func(ctx context.Context, args map[string]any) (string, error) // 真正执行的函数
}

// ToParam 把工具转成 openai-go 需要的格式
func (t ToolDef) ToParam() responses.ToolUnionParam {
	return responses.ToolUnionParam{
		OfFunction: &responses.FunctionToolParam{
			Name:        t.Name,
			Description: openai.String(t.Description),
			Parameters:  t.Params,
		},
	}
}

// Call 大模型传回来的参数是一段 JSON 字符串，
// 这里先解析成 map，再调用真正的 Go 函数 Fn
func (t ToolDef) Call(ctx context.Context, argsJSON string) (string, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", err
	}
	return t.Fn(ctx, args)
}
