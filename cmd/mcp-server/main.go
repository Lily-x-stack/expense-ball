package main

// MCP Server 的独立可执行入口：通过 stdio（标准输入输出）和 MCP Client 通信。
// 它本身不打印任何界面，所有输入输出都是 MCP 协议消息。
import (
	"fmt"

	"expense-ball/mcp"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	if err := server.ServeStdio(mcp.NewServer()); err != nil {
		fmt.Printf("MCP server error: %v\n", err)
	}
}
