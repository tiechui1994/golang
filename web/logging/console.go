package logging

import (
	"encoding/json"
	"os"
	"runtime"
	"time"
)

type brush func(string) string

// 设置输出日志的颜色, "\033[1;xxm" + msg + "\033[0m"
func newBrush(color string) brush {
	pre := "\033["
	reset := "\033[0m"
	return func(text string) string {
		return pre + color + "m" + text + reset
	}
}

// 颜色数组
var colors = []brush{
	newBrush("1;37"), // Emergency          white
	newBrush("1;36"), // Alert              cyan
	newBrush("1;35"), // Critical           magenta
	newBrush("1;31"), // Error              red
	newBrush("1;33"), // Warning            yellow
	newBrush("1;32"), // Notice             green
	newBrush("1;34"), // Informational      blue
	newBrush("1;44"), // Debug              Background blue
}

// 核心是控制终端的颜色显示
// 实现了Logger, 即Console引擎(终端日志)
type consoleWriter struct {
	lg       *logWriter
	Level    int  `json:"level"`
	Colorful bool `json:"color"` // 仅当系统终端支持颜色时, 此字段才有用
}

func NewConsole() Logger {
	cw := &consoleWriter{
		lg:       newLogWriter(os.Stdout), // 将标准输出作为Writer
		Level:    LevelDebug,
		Colorful: runtime.GOOS != "windows", // 颜色控制
	}
	return cw
}

// 初始化, 设置参数(2个)
// jsonConfig like '{"level":LevelTrace}'.
func (c *consoleWriter) Init(jsonConfig string) error {
	if len(jsonConfig) == 0 {
		return nil
	}
	err := json.Unmarshal([]byte(jsonConfig), c)
	if runtime.GOOS == "windows" {
		c.Colorful = false
	}
	return err
}

// 写消息:
func (c *consoleWriter) WriteMsg(when time.Time, msg string, level int) error {
	if level > c.Level {
		return nil
	}

	// 颜色控制
	if c.Colorful {
		msg = colors[level](msg)
	}

	c.lg.println(when, msg)
	return nil
}

func (c *consoleWriter) Destroy() {

}

func (c *consoleWriter) Flush() {

}

func init() {
	Register(AdapterConsole, NewConsole)
}
