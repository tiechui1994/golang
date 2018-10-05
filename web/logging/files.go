package logging

import (
	"time"
	"strings"
	"path/filepath"
	"os"
	"sync"
	"strconv"
	"path"
	"io"
	"bytes"
	"fmt"
	"errors"
	"encoding/json"
)

// 文件日志, 核心是将日志写入到文件当中, 并且还要应对复杂情况下日志文件分割
// 实现了Logger接口
type fileLogWriter struct {
	sync.RWMutex // 写入日志时原子性修改 maxLinesCurLines 和 maxSizeCurSize 变量
	Level int `json:"level"`

	// 写入的文件信息
	Filename             string `json:"filename"`
	fileWriter           *os.File
	fileNameOnly, suffix string // 日志文件的前缀和后缀

	// 单文件最大行数, 最大文件数量, 单文件最大容量
	MaxLines         int `json:"maxlines"`
	maxLinesCurLines int

	MaxFiles         int `json:"maxfiles"`
	MaxFilesCurFiles int

	MaxSize        int `json:"maxsize"`
	maxSizeCurSize int

	// 按照天数进行轮询
	Daily         bool  `json:"daily"`
	MaxDays       int64 `json:"maxdays"`
	dailyOpenDate int       // 文件打开日期
	dailyOpenTime time.Time // 文件打开时间

	// 按照小时进行轮询
	Hourly         bool  `json:"hourly"`
	MaxHours       int64 `json:"maxhours"`
	hourlyOpenDate int
	hourlyOpenTime time.Time

	// 是否进行轮询
	Rotate bool `json:"rotate"`

	// 当前日志文件权限
	Perm string `json:"perm"`

	// 旧日志文件权限
	RotatePerm string `json:"rotateperm"`
}

// 日志的默认配置
func newFileWriter() Logger {
	w := &fileLogWriter{
		Level: LevelTrace,

		// 两者选其一
		Daily:    true,
		MaxDays:  7,
		Hourly:   false,
		MaxHours: 168,

		Rotate:     true,   // 按照天数进行轮询
		RotatePerm: "0440", // 旧日志权限
		Perm:       "0660", // 当前操作文件权限

		MaxLines: 10000000,
		MaxFiles: 999,
		MaxSize:  1 << 28,
	}
	return w
}

/*
默认的配置参数:
{
  "suffix": ".log"

  "level": 7,

  "maxLines":10000000,
  "maxsize":1 << 28,
  "maxFiles": 999,

  "rotateperm": "0440",
  "perm":"0660",

  "rotate":true,
  "daily":true,
  "maxDays":7,
}
*/
// 必须要的参数: Filename
func (w *fileLogWriter) Init(jsonConfig string) error {
	err := json.Unmarshal([]byte(jsonConfig), w)
	if err != nil {
		return err
	}
	if len(w.Filename) == 0 {
		return errors.New("jsonconfig must have filename")
	}
	w.suffix = filepath.Ext(w.Filename)
	w.fileNameOnly = strings.TrimSuffix(w.Filename, w.suffix)
	if w.suffix == "" {
		w.suffix = ".log"
	}
	err = w.startLogger()
	return err
}

// 启动文件日志: 创建文件, 初始化文件
func (w *fileLogWriter) startLogger() error {
	file, err := w.createLogFile() // ...
	if err != nil {
		return err
	}
	if w.fileWriter != nil {
		w.fileWriter.Close()
	}
	w.fileWriter = file
	return w.initFd() // ...
}

// 创建log文件, 并且调整其权限为设置的权限
func (w *fileLogWriter) createLogFile() (*os.File, error) {
	// log文件权限解析(8进制), "0666"
	perm, err := strconv.ParseInt(w.Perm, 8, 64)
	if err != nil {
		return nil, err
	}

	// log文件路径解析
	filepath := path.Dir(w.Filename)
	os.MkdirAll(filepath, os.FileMode(perm))

	fd, err := os.OpenFile(w.Filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, os.FileMode(perm))
	if err == nil {
		// 确保文件权限正确
		os.Chmod(w.Filename, os.FileMode(perm))
	}
	return fd, err
}

// 初始化文件相关的变量, 并且开启 goroutine 进行异步处理消息
func (w *fileLogWriter) initFd() error {
	fd := w.fileWriter
	fInfo, err := fd.Stat() // 文件的详情
	if err != nil {
		return fmt.Errorf("get stat err: %s", err)
	}

	w.dailyOpenTime = time.Now()
	w.dailyOpenDate = w.dailyOpenTime.Day() // 天
	w.hourlyOpenTime = time.Now()
	w.hourlyOpenDate = w.hourlyOpenTime.Hour() // 小时

	w.maxSizeCurSize = int(fInfo.Size())
	w.maxLinesCurLines = 0

	if w.Hourly {
		go w.hourlyRotate(w.hourlyOpenTime)
	} else if w.Daily {
		go w.dailyRotate(w.dailyOpenTime)
	}

	// 由于打开的文件可能已经已经存在, 这里需要调整初始化的最值
	if fInfo.Size() > 0 && w.MaxLines > 0 {
		count, err := w.lines()
		if err != nil {
			return err
		}
		w.maxLinesCurLines = count
	}

	return nil
}

// 统计文件的行数
func (w *fileLogWriter) lines() (int, error) {
	fd, err := os.Open(w.Filename)
	if err != nil {
		return 0, err
	}
	defer fd.Close()

	count := 0
	lineSep := []byte{'\n'}    // 分行符
	buf := make([]byte, 32768) // 32k

	for {
		c, err := fd.Read(buf)
		if err != nil && err != io.EOF {
			return count, err
		}

		count += bytes.Count(buf[:c], lineSep) // 统计分行符的总数

		if err == io.EOF {
			break
		}
	}

	return count, nil
}

// 销毁日志, 关闭fd
func (w *fileLogWriter) Destroy() {
	w.fileWriter.Close()
}

// 文件刷新操作: 将缓存当中日志同步到硬盘上
func (w *fileLogWriter) Flush() {
	w.fileWriter.Sync()
}

//----------------------------------------------------------------------------------------------------------------------
// 文件引擎的精彩部分: 日志切换(写日志导致的操作)

// 异步轮训(定时器): 开启一个goroutine, 定时进行文件切换.
func (w *fileLogWriter) dailyRotate(openTime time.Time) {
	// 计算下一次文件切换的时间
	y, m, d := openTime.Add(24 * time.Hour).Date()
	nextDay := time.Date(y, m, d, 0, 0, 0, 0, openTime.Location())

	// 构建定时器, 并阻塞等待,直到切换的时间到达
	tm := time.NewTimer(time.Duration(nextDay.UnixNano() - openTime.UnixNano() + 100))
	<-tm.C

	// 文件切换
	w.Lock()
	if w.needRotateDaily(0, time.Now().Day()) {
		if err := w.doRotate(time.Now()); err != nil {
			fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.Filename, err)
		}
	}
	w.Unlock()
}

// 原理同上
func (w *fileLogWriter) hourlyRotate(openTime time.Time) {
	y, m, d := openTime.Add(1 * time.Hour).Date()
	h, _, _ := openTime.Add(1 * time.Hour).Clock()
	nextHour := time.Date(y, m, d, h, 0, 0, 0, openTime.Location())
	tm := time.NewTimer(time.Duration(nextHour.UnixNano() - openTime.UnixNano() + 100))
	<-tm.C
	w.Lock()
	if w.needRotateHourly(0, time.Now().Hour()) {
		if err := w.doRotate(time.Now()); err != nil {
			fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.Filename, err)
		}
	}
	w.Unlock()
}

// 写入消息, 会导致文件切换
func (w *fileLogWriter) WriteMsg(when time.Time, msg string, level int) error {
	if level > w.Level {
		return nil
	}

	// 日志内容
	hd, d, h := formatTimeHeader(when)
	msg = string(hd) + msg + "\n"

	// 日志切换判断(判断是可重入锁, 切换是不可重入锁)
	if w.Rotate {
		w.RLock()
		// 应对高并发情况:
		// 第一次判断确定是否需要切换 -> 需要切换,第二次判断是否已经切换 -> 切换
		if w.needRotateHourly(len(msg), h) {
			w.RUnlock()
			w.Lock()
			if w.needRotateHourly(len(msg), h) {
				if err := w.doRotate(when); err != nil {
					fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.Filename, err)
				}
			}
			w.Unlock()
		} else if w.needRotateDaily(len(msg), d) {
			w.RUnlock()
			w.Lock()
			if w.needRotateDaily(len(msg), d) {
				if err := w.doRotate(when); err != nil {
					fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.Filename, err)
				}
			}
			w.Unlock()
		} else {
			w.RUnlock()
		}
	}

	// 写入日志
	w.Lock()
	_, err := w.fileWriter.Write([]byte(msg))
	if err == nil {
		w.maxLinesCurLines++
		w.maxSizeCurSize += len(msg)
	}
	w.Unlock()

	return err
}

// 文件是否需要切换判断: 单个文件最大行, 单个文件大小, 时间
func (w *fileLogWriter) needRotateDaily(size int, day int) bool {
	return (w.MaxLines > 0 && w.maxLinesCurLines >= w.MaxLines) ||
		(w.MaxSize > 0 && w.maxSizeCurSize >= w.MaxSize) ||
		(w.Daily && day != w.dailyOpenDate)
}

func (w *fileLogWriter) needRotateHourly(size int, hour int) bool {
	return (w.MaxLines > 0 && w.maxLinesCurLines >= w.MaxLines) ||
		(w.MaxSize > 0 && w.maxSizeCurSize >= w.MaxSize) ||
		(w.Hourly && hour != w.hourlyOpenDate)

}

// 切换日志文件: (原子性操作: 构建新文件 -> 重命名)
// 新的日志文件名称 xx.2013-01-01.log (daily) or xx.001.log (line or size)
func (w *fileLogWriter) doRotate(logTime time.Time) error {
	var (
		fName, format string
		openTime      time.Time
	)
	// 文件编号
	num := w.MaxFilesCurFiles + 1
	rotatePerm, err := strconv.ParseInt(w.RotatePerm, 8, 64)
	if err != nil {
		return err
	}

	// 确保当前log文件存在
	_, err = os.Lstat(w.Filename)
	if err != nil {
		// 文件不存在或者其难以预测的问题
		goto RESTART_LOGGER
	}

	if w.Hourly {
		format = "2006010215"
		openTime = w.hourlyOpenTime
	} else if w.Daily {
		format = "2006-01-02"
		openTime = w.dailyOpenTime
	}

	// 获取文件名称
	if w.MaxLines > 0 || w.MaxSize > 0 { // 具有文件大小, 行数有限制(当前时间, 需要遍历所有的文件编号)
		for ; err == nil && num <= w.MaxFiles; num++ {
			fName = w.fileNameOnly + fmt.Sprintf(".%s.%03d%s", logTime.Format(format), num, w.suffix)
			_, err = os.Lstat(fName) // 获取当前序列号文件的状态
		}
	} else { // 文件大小没有限制(切换时间, 文件编号严格递增)
		fName = w.fileNameOnly + fmt.Sprintf(".%s.%03d%s", openTime.Format(format), num, w.suffix)
		_, err = os.Lstat(fName)
		w.MaxFilesCurFiles = num
	}

	// 说明num已经到达上线,但是没有找到合适的文件编号, 只能记录在当前文件了
	if err == nil {
		return fmt.Errorf("Rotate: Cannot find free log number to rename %s", w.Filename)
	}

	// 文件切换: 关闭当前文件, 对当前文件重命名, 更新重命名后的文件权限, 重启
	w.fileWriter.Close()
	err = os.Rename(w.Filename, fName) // 系统调用,原子操作
	if err != nil {
		goto RESTART_LOGGER
	}
	err = os.Chmod(fName, os.FileMode(rotatePerm)) // 修改历史的文件的权限

	//在命名失败 或者 文件被删除, 正常执行到此都会执行
RESTART_LOGGER:
	startLoggerErr := w.startLogger() // 重新启动log
	go w.deleteOldLog()               // 删除旧文件

	if startLoggerErr != nil {
		return fmt.Errorf("Rotate StartLogger: %s", startLoggerErr)
	}
	if err != nil {
		return fmt.Errorf("Rotate: %s", err)
	}
	return nil
}

// 删除文件, 会自动删除过了一个轮训时间的日志文件
func (w *fileLogWriter) deleteOldLog() {
	dir := filepath.Dir(w.Filename)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) (returnErr error) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "Unable to delete old log '%s', error: %v\n", path, r)
			}
		}()

		if info == nil {
			return
		}
		if w.Hourly {
			if !info.IsDir() && info.ModTime().Add(1 * time.Hour * time.Duration(w.MaxHours)).Before(time.Now()) {
				if strings.HasPrefix(filepath.Base(path), filepath.Base(w.fileNameOnly)) &&
					strings.HasSuffix(filepath.Base(path), w.suffix) {
					os.Remove(path)
				}
			}
		} else if w.Daily {
			if !info.IsDir() && info.ModTime().Add(24 * time.Hour * time.Duration(w.MaxDays)).Before(time.Now()) {
				if strings.HasPrefix(filepath.Base(path), filepath.Base(w.fileNameOnly)) &&
					strings.HasSuffix(filepath.Base(path), w.suffix) {
					os.Remove(path)
				}
			}
		}
		return
	})
}

func init() {
	Register(AdapterFile, newFileWriter)
}
