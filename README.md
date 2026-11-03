# 记账助手（expense-ball）

一个用 Go 写的小型 AI Agent 记账项目：在终端里用大白话记账，大模型自动理解收入/支出和分类，数据存在本地 JSON 文件里。
界面使用 [Bubble Tea](https://github.com/charmbracelet/bubbletea) 终端 UI 框架，工具能力通过 **MCP（Model Context Protocol）** 接入。

## 能做什么

- 记账：「今天午饭花了15」「昨天打车22」「收到兼职工资500」
- 查账：「查一下今天的账」「看看这个月的收支」
- 自动按分类汇总：
  - 支出：餐饮 15.00 元、交通 22.00 元……支出合计 37.50 元
  - 收入：兼职 500.00 元……收入合计 500.00 元
  - 结余（收入-支出）：462.50 元

## MCP 架构

工具不再写死在主程序里，而是拆成独立的 MCP Server，主程序作为 Client 启动时动态发现。

```
┌────────────────────────────────────────┐
│  Host：expense-ball（终端 TUI）         │
│    ┌────────────────────────────────┐  │
│    │ MCP Client（agent.go）          │  │
│    │   ListTools 动态发现工具        │  │
│    │   CallTool 让 Server 执行       │  │
│    └───────────────┬────────────────┘  │
└────────────────────┼───────────────────┘
                     │ stdio（标准输入输出，JSON-RPC 2.0）
┌────────────────────┴───────────────────┐
│  MCP Server：expense-mcp-server         │
│    工具：add_record / query_records     │
│       ↓ 调用                             │
│  tools 业务层 → data/records.json       │
└────────────────────────────────────────┘
```

- 用 [mcp-go](https://github.com/mark3labs/mcp-go) 实现，传输方式为 **stdio 本地子进程**，不开端口。
- 主程序启动时会把 MCP Server 作为子进程拉起；退出时自动关闭。
- 大模型本身只输出「要调用哪个工具、参数是什么」，真正读写文件的是 MCP Server 里的 Go 代码。

## 运行前：配置 API Key（必做）

代码不内置任何 key，必须通过环境变量 `ARK_API_KEY` 提供，否则程序启动会直接报错。

PowerShell 里执行（只对当前窗口有效）：

```powershell
$env:ARK_API_KEY="你的新key"
```

## 运行（重要：先编译 MCP Server）

主程序启动时会去找当前目录下的 MCP Server，所以要先把它编译出来。

```powershell
# 下载依赖（第一次运行需要）
go mod tidy

# 1. 编译 MCP Server（产物在项目根目录）
go build -o expense-mcp-server.exe ./cmd/mcp-server

# 2. 编译并运行主程序
go build -o expense-ball.exe .
.\expense-ball.exe
```

开发时也可以不编译主程序，直接：

```powershell
go build -o expense-mcp-server.exe ./cmd/mcp-server
go run .
```

> 注意：请在**项目根目录**运行。MCP Server 子进程会继承当前工作目录，
> 这样 `data/records.json` 的相对路径才一致。
>
> 如果 Server 不在默认位置，可用环境变量指定启动命令：
> ```powershell
> $env:MCP_SERVER_CMD="./expense-mcp-server.exe"
> ```

运行后：
- 在输入框打字，回车发送
- 灰色文字是大模型的思考和工具调用过程
- 输入 `quit` 或按 `Ctrl+C` 退出（会同时关闭 MCP Server 子进程）

## 测试

```powershell
go test ./...
```

单元测试不依赖大模型，直接测 `tools` 包的记账和查询逻辑。

## 项目结构

```
expense-ball/
├── main.go                  # Bubble Tea 界面（model-update-view 三件套）
├── agent.go                 # MCP Client + Agent 循环：连接 Server、动态发现工具、调度工具
├── core/
│   └── session.go           # 会话历史
├── mcp/
│   └── server.go            # 构建 MCP Server，注册两个工具
├── cmd/
│   └── mcp-server/
│       └── main.go          # MCP Server 可执行入口（ServeStdio）
├── tools/
│   ├── tool.go              # 工具定义基类 ToolDef
│   ├── service.go           # 业务逻辑层：AddRecord / QueryRecords
│   ├── store.go             # 数据存储：读写 data/records.json
│   ├── add_record.go        # 工具1 定义：记一笔收入/支出
│   ├── query_records.go     # 工具2 定义：按天/月查询、分类汇总
│   └── tools_test.go        # 工具层单元测试
├── data/
│   └── records.json         # 账目数据
└── go.mod
```

## 一笔数据长什么样

```json
{
  "type": "expense",        // expense=支出，income=收入
  "amount": 15,             // 金额
  "category": "餐饮",        // 分类
  "note": "午饭",           // 备注
  "date": "2026-10-14"      // 日期
}
```

## 之后可以自己加的功能（练手）

- 删除/修改记录（在 MCP Server 里加一个 `delete_record` 工具，主程序无需改分发逻辑）
- 月度预算超支提醒
- 数据改成 SQLite 存储
- 界面里加一个本月收支统计面板
- 把 MCP Server 改成 Streamable HTTP 传输，支持远程调用
