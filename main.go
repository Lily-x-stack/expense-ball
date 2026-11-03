package main

// Bubble Tea 终端界面（TUI）。
// Bubble Tea 的核心是 Elm 架构，分三部分：
//   model：保存界面状态（消息列表、输入框内容、是否正在思考）
//   update：收到事件（按键、后台消息）时更新状态
//   view：根据状态画出界面

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// 界面样式
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("213")) // 粉色标题
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")) // 青色：用户说的话
	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")) // 绿色：助手回答
	logStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")) // 灰色：工具调用过程
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")) // 红色：报错
)

// agentEvent 是后台运行的 Agent 通过 channel 发给界面的消息
type agentEvent struct {
	kind string // "log" 过程日志 / "done" 最终回答 / "error" 出错
	text string
}

type model struct {
	viewport viewport.Model  // 消息滚动区
	input    textinput.Model // 底部输入框
	agent    *Agent
	eventCh  chan agentEvent // 接收 Agent 运行过程的通道
	running  bool            // Agent 是否正在思考
	width    int
	height   int
	lines    []string // 消息区所有已经渲染好的行
}

func initialModel() model {
	// 输入框
	ti := textinput.New()
	ti.Placeholder = "例如：今天午饭花了15"
	ti.Focus() // 启动后光标直接在输入框里

	// 消息滚动区（宽高后面拿到窗口尺寸再更新）
	vp := viewport.New(80, 20)

	// 启动 Agent：连接 MCP Server 并动态发现工具
	agent, err := NewAgent()
	if err != nil {
		panic(err)
	}

	m := model{
		viewport: vp,
		input:    ti,
		agent:    agent,
	}

	// 欢迎语
	m.addMsg("assistant",
		"你好，我是记账助手。\n"+
			"你可以这样对我说：\n"+
			"  · 今天午饭花了15\n"+
			"  · 昨天打车22块\n"+
			"  · 收到兼职工资500\n"+
			"  · 查一下今天的账\n"+
			"  · 看看这个月的收支\n"+
			"输入 quit 退出程序。")
	return m
}

// Init 程序启动时要执行的命令；这里不需要做什么
func (m model) Init() tea.Cmd {
	return textinput.Blink // 让输入框光标闪烁
}

// Update 处理所有"事件"：按键、窗口缩放、Agent 后台发来的消息
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg: // 窗口大小变化时，重新计算布局
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 3 // 标题栏 + 状态行 + 输入框 共占 3 行
		m.input.Width = msg.Width - 4
		m.refresh()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			// Agent 思考时忽略新的回车
			if m.running {
				return m, nil
			}
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}
			if text == "quit" || text == "exit" {
				return m, tea.Quit
			}

			// 1. 界面显示用户这句话
			m.addMsg("user", text)
			m.input.Reset()
			m.running = true

			// 2. 开一个 channel，Agent 在后台 goroutine 里运行，
			//    思考过程和最终结果都通过这个 channel 发回来
			ch := make(chan agentEvent, 64)
			m.eventCh = ch
			go func(question string) {
				// logf 回调：Agent 每产生一条过程日志，就往 channel 里塞一条
				reply, err := m.agent.Run(context.Background(), question, func(s string) {
					ch <- agentEvent{kind: "log", text: s}
				})
				if err != nil {
					ch <- agentEvent{kind: "error", text: err.Error()}
				} else {
					ch <- agentEvent{kind: "done", text: reply}
				}
			}(text)

			// 3. 告诉 Bubble Tea：等 channel 里的下一条消息
			return m, waitAgent(ch)
		}

	case agentEvent: // 收到 Agent 从后台发来的消息
		switch msg.kind {
		case "log":
			m.addMsg("log", msg.text)
			// Agent 还没结束，继续等下一条
			return m, waitAgent(m.eventCh)
		case "done":
			m.addMsg("assistant", msg.text)
			m.running = false
			return m, nil
		case "error":
			m.addMsg("error", "出错了："+msg.text)
			m.running = false
			return m, nil
		}
	}

	// 其他事件交给输入框和滚动区自己处理（比如输入字符、翻页）
	var cmd tea.Cmd
	var cmds []tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

// waitAgent 等待 channel 里的一条 Agent 消息
func waitAgent(ch chan agentEvent) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

// addMsg 追加一条消息到消息区
// role: user / assistant / log / error
func (m *model) addMsg(role, text string) {
	// 长文本按界面宽度自动换行
	wrapped := wrapText(text, m.viewport.Width-2)
	for _, line := range strings.Split(wrapped, "\n") {
		switch role {
		case "user":
			m.lines = append(m.lines, userStyle.Render("你："+line))
		case "assistant":
			m.lines = append(m.lines, assistantStyle.Render("助手："+line))
		case "log":
			m.lines = append(m.lines, logStyle.Render("  · "+line))
		case "error":
			m.lines = append(m.lines, errorStyle.Render("✗ "+line))
		}
	}
	// 每条消息之间加一个空行
	m.lines = append(m.lines, "")
	m.refresh()
}

// refresh 重新渲染消息区并滚动到最底部
func (m *model) refresh() {
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
	m.viewport.GotoBottom()
}

// View 画出整个界面
func (m model) View() string {
	// 状态行：思考时显示提示，否则显示用法
	statusLine := ""
	if m.running {
		statusLine = logStyle.Render("思考中，请稍候...")
	} else {
		statusLine = logStyle.Render("输入账目或问题，回车发送；输入 quit 退出")
	}

	return strings.Join([]string{
		titleStyle.Render("── 记账助手 ──"),
		m.viewport.View(),
		statusLine,
		m.input.View(),
	}, "\n")
}

// wrapText 按宽度对文本换行（中文按一个字符算，够用且简单）
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	var result strings.Builder
	for _, paragraph := range strings.Split(text, "\n") {
		count := 0
		for _, r := range paragraph {
			result.WriteRune(r)
			count++
			if count >= width {
				result.WriteString("\n")
				count = 0
			}
		}
		result.WriteString("\n")
	}
	return strings.TrimRight(result.String(), "\n")
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	finalModel, err := p.Run()
	// 退出前关闭 MCP 连接，结束 server 子进程
	if m, ok := finalModel.(model); ok {
		_ = m.agent.Close()
	}
	if err != nil {
		panic(err)
	}
}
