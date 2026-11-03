package core

// Session 负责保存整个对话过程中的所有消息
// （系统提示词、用户的话、助手的话、工具调用和工具返回结果）
import (
	"github.com/openai/openai-go/v3/responses"
)

type Session struct {
	Messages responses.ResponseInputParam // 消息列表
	Len      int                          // 消息条数（仅作计数用）
}

func NewSession() *Session {
	return &Session{
		Messages: responses.ResponseInputParam{},
		Len:      0,
	}
}

// GetMessages 返回全部消息，每次调用大模型时传进去
func (s *Session) GetMessages() responses.ResponseInputParam {
	return s.Messages
}

// AddMessage 追加一条或多条消息
func (s *Session) AddMessage(item ...responses.ResponseInputItemUnionParam) {
	s.Messages = append(s.Messages, item...)
	s.Len++
}
