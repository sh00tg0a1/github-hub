package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// LocalTime 是一个自定义时间类型，用于处理JSON序列化时使用本地时区（Asia/Shanghai）
type LocalTime struct {
	time.Time
}

// 定义上海时区
var shanghaiLoc *time.Location

func init() {
	var err error
	shanghaiLoc, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		// 如果加载失败，使用固定的 +8 小时偏移
		shanghaiLoc = time.FixedZone("CST", 8*60*60)
	}
}

// Now 返回当前本地时间
func Now() LocalTime {
	return LocalTime{time.Now().In(shanghaiLoc)}
}

// ParseLocalTime 解析字符串为 LocalTime
func ParseLocalTime(s string) (LocalTime, error) {
	if s == "" || s == "null" {
		return LocalTime{}, nil
	}

	// 尝试多种时间格式
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.999999999Z",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
	}

	var t time.Time
	var err error
	for _, format := range formats {
		t, err = time.ParseInLocation(format, s, shanghaiLoc)
		if err == nil {
			return LocalTime{t}, nil
		}
	}

	// 如果所有格式都失败，尝试直接解析
	t, err = time.Parse(time.RFC3339, s)
	if err == nil {
		return LocalTime{t.In(shanghaiLoc)}, nil
	}

	return LocalTime{}, fmt.Errorf("unable to parse time: %s", s)
}

// MarshalJSON 实现 json.Marshaler 接口
func (lt LocalTime) MarshalJSON() ([]byte, error) {
	if lt.Time.IsZero() {
		return []byte("null"), nil
	}

	// 转换为上海时区
	localTime := lt.Time.In(shanghaiLoc)

	// 格式化为带时区偏移的格式 (类似 RFC3339 但不使用 Z)
	// 上海时区是 UTC+8，所以格式为: 2006-01-02T15:04:05+08:00
	formatted := localTime.Format("2006-01-02T15:04:05-07:00")
	return json.Marshal(formatted)
}

// UnmarshalJSON 实现 json.Unmarshaler 接口
func (lt *LocalTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		lt.Time = time.Time{}
		return nil
	}

	// 去掉引号
	s := strings.Trim(string(data), `"`)
	if s == "" {
		lt.Time = time.Time{}
		return nil
	}

	// 尝试多种格式
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05Z",
	}

	var t time.Time
	var err error
	for _, format := range formats {
		t, err = time.Parse(format, s)
		if err == nil {
			lt.Time = t.In(shanghaiLoc)
			return nil
		}
	}

	return fmt.Errorf("unable to parse time: %s", s)
}

// Value 实现 driver.Valuer 接口，用于数据库存储
func (lt LocalTime) Value() (driver.Value, error) {
	return lt.Time, nil
}

// Scan 实现 sql.Scanner 接口，用于数据库读取
func (lt *LocalTime) Scan(value interface{}) error {
	if value == nil {
		lt.Time = time.Time{}
		return nil
	}

	if t, ok := value.(time.Time); ok {
		lt.Time = t.In(shanghaiLoc)
		return nil
	}

	return fmt.Errorf("cannot scan %T into LocalTime", value)
}

// String 返回字符串表示
func (lt LocalTime) String() string {
	if lt.Time.IsZero() {
		return ""
	}
	return lt.Time.In(shanghaiLoc).Format("2006-01-02 15:04:05")
}

// Format 按指定格式返回字符串
func (lt LocalTime) Format(layout string) string {
	if lt.Time.IsZero() {
		return ""
	}
	return lt.Time.In(shanghaiLoc).Format(layout)
}

// IsZero 判断是否为零值
func (lt LocalTime) IsZero() bool {
	return lt.Time.IsZero()
}

// ToTime 转换为标准 time.Time
func (lt LocalTime) ToTime() time.Time {
	return lt.Time
}

// FromTime 从标准 time.Time 创建 LocalTime
func FromTime(t time.Time) LocalTime {
	return LocalTime{t.In(shanghaiLoc)}
}
