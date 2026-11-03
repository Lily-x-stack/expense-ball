package tools

// 数据存储层：把所有账目保存在 data/records.json 文件里。
// 不用数据库，只用标准库的 encoding/json 读写文件。
import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const dataFile = "data/records.json"

// Record 是一条账目记录
type Record struct {
	Type     string  `json:"type"`     // 类型：expense=支出，income=收入
	Amount   float64 `json:"amount"`   // 金额
	Category string  `json:"category"` // 分类：餐饮、交通、工资、红包 等
	Note     string  `json:"note"`     // 备注：午饭、打车 等
	Date     string  `json:"date"`     // 日期：2006-01-02 格式
}

// 用一把锁保护文件，避免同时读写造成数据混乱
var fileMu sync.Mutex

// loadRecords 读取全部记录；如果文件还不存在，就返回空切片
func loadRecords() ([]Record, error) {
	fileMu.Lock()
	defer fileMu.Unlock()

	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []Record{}, nil // 第一次运行，文件还没生成
		}
		return nil, err
	}

	var records []Record
	if len(data) == 0 {
		return []Record{}, nil
	}
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// saveRecords 把全部记录写回文件（缩进格式，方便人直接打开看）
func saveRecords(records []Record) error {
	fileMu.Lock()
	defer fileMu.Unlock()

	// 确保 data 目录存在
	if err := os.MkdirAll(filepath.Dir(dataFile), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dataFile, data, 0o644)
}
