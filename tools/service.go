package tools

// 业务逻辑层：记账和查账的真正实现。
// MCP Server（mcp/server.go）和本地工具定义（add_record.go、query_records.go）
// 都调用这里的函数，保证两条路径共用同一套逻辑、不重复代码。
import (
	"context"
	"fmt"
	"strings"
	"time"
)

// AddRecord 记录一笔收入或支出
func AddRecord(ctx context.Context, args map[string]any) (string, error) {
	// 1. 解析类型
	recordType, _ := args["type"].(string)
	if recordType != "expense" && recordType != "income" {
		return "", fmt.Errorf("type 只能是 expense 或 income")
	}

	// 2. 解析金额（JSON 里的数字解析出来是 float64）
	amount, ok := args["amount"].(float64)
	if !ok || amount <= 0 {
		return "", fmt.Errorf("金额必须是正数")
	}

	// 3. 解析分类和备注
	category, _ := args["category"].(string)
	note, _ := args["note"].(string)

	// 4. 日期没传就用今天
	date, _ := args["date"].(string)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// 5. 读取旧记录 → 追加新记录 → 写回文件
	records, err := loadRecords()
	if err != nil {
		return "", err
	}
	records = append(records, Record{
		Type:     recordType,
		Amount:   amount,
		Category: category,
		Note:     note,
		Date:     date,
	})
	if err := saveRecords(records); err != nil {
		return "", err
	}

	typeText := "支出"
	if recordType == "income" {
		typeText = "收入"
	}
	return fmt.Sprintf("已记录：%s %s %.2f 元（%s）日期 %s",
		typeText, category, amount, note, date), nil
}

// QueryRecords 按天或按月查询账目，按分类汇总收入和支出
func QueryRecords(ctx context.Context, args map[string]any) (string, error) {
	date, _ := args["date"].(string)
	month, _ := args["month"].(string)

	// 1. 读取全部记录
	records, err := loadRecords()
	if err != nil {
		return "", err
	}

	// 2. 按条件过滤：日期前缀匹配（某天 / 整月）
	prefix := ""
	rangeText := "全部记录"
	switch {
	case date != "":
		prefix = date
		rangeText = date + " 当天"
	case month != "":
		prefix = month
		rangeText = month + " 整月"
	}

	var filtered []Record
	for _, r := range records {
		if prefix == "" || strings.HasPrefix(r.Date, prefix) {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) == 0 {
		return rangeText + " 没有账目记录", nil
	}

	// 3. 分别累加支出、收入，并按分类汇总
	var expenseLines, incomeLines []string
	expenseByCategory := map[string]float64{}
	incomeByCategory := map[string]float64{}
	var totalExpense, totalIncome float64

	for _, r := range filtered {
		line := fmt.Sprintf("- %.2f 元 [%s] %s %s",
			r.Amount, r.Category, r.Note, r.Date)

		switch r.Type {
		case "expense":
			expenseLines = append(expenseLines, line)
			expenseByCategory[r.Category] += r.Amount
			totalExpense += r.Amount
		case "income":
			incomeLines = append(incomeLines, line)
			incomeByCategory[r.Category] += r.Amount
			totalIncome += r.Amount
		}
	}

	// 4. 拼接结果文本
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s，共 %d 笔记录\n", rangeText, len(filtered))

	sb.WriteString("\n【支出明细】\n")
	if len(expenseLines) == 0 {
		sb.WriteString("无\n")
	} else {
		sb.WriteString(strings.Join(expenseLines, "\n") + "\n")
	}

	sb.WriteString("\n【收入明细】\n")
	if len(incomeLines) == 0 {
		sb.WriteString("无\n")
	} else {
		sb.WriteString(strings.Join(incomeLines, "\n") + "\n")
	}

	sb.WriteString("\n【支出分类汇总】\n")
	for category, amount := range expenseByCategory {
		fmt.Fprintf(&sb, "%s: %.2f 元\n", category, amount)
	}
	fmt.Fprintf(&sb, "支出合计: %.2f 元\n", totalExpense)

	sb.WriteString("\n【收入分类汇总】\n")
	for category, amount := range incomeByCategory {
		fmt.Fprintf(&sb, "%s: %.2f 元\n", category, amount)
	}
	fmt.Fprintf(&sb, "收入合计: %.2f 元\n", totalIncome)

	fmt.Fprintf(&sb, "\n结余（收入-支出）: %.2f 元\n", totalIncome-totalExpense)

	return sb.String(), nil
}
