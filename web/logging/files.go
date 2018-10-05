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

// 实现了Logger接口
type fileLogWriter struct {
	sync.RWMutex // 写入日志时原子性修改 maxLinesCurLines 和 maxSizeCurSize 变量
	// 写入的文件信息
	Filename   string `json:"filename"`
	fileWriter *os.File

	// 单个文件行数, 最多文件数量, 单个文件大小的限制
	MaxLines         int `json:"maxlines"`
	maxLinesCurLines int

	MaxFiles         int `json:"maxfiles"`
	MaxFilesCurFiles int

	MaxSize        int `json:"maxsize"`
	maxSizeCurSize int

	// 按照天数进行轮询
	Daily         bool  `json:"daily"`
	MaxDays       int64 `json:"maxdays"`
	dailyOpenDate int
	dailyOpenTime time.Time

	// 按照小时进行轮询
	Hourly         bool  `json:"hourly"`
	MaxHours       int64 `json:"maxhours"`
	hourlyOpenDate int
	hourlyOpenTime time.Time

	// 是否进行轮询
	Rotate bool `json:"rotate"`

	Level int `json:"level"`

	// 文件权限
	Perm string `json:"perm"`

	// 已经轮询的文件的权限
	RotatePerm string `json:"rotateperm"`

	fileNameOnly, suffix string // like "project.log", project is fileNameOnly and .log is suffix
}

// newFileWriter create a FileLogWriter returning as LoggerInterface.
func newFileWriter() Logger {
	w := &fileLogWriter{
		Daily:      true,
		MaxDays:    7,
		Hourly:     false,
		MaxHours:   168,
		Rotate:     true,  //按照天数进行轮询
		RotatePerm: "0440",
		Level:      LevelTrace,
		Perm:       "0660",
		MaxLines:   10000000,
		MaxFiles:   999,
		MaxSize:    1 << 28,
	}
	return w
}

/*
参数配置例子: 传入的是String
{
  "filename":"logs/beego.log",
  "maxLines":10000,
  "maxsize":1024,
  "daily":true,
  "maxDays":15,
  "rotate":true,
  "perm":"0600"
}
*/
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

// start file logger. create log file and set to locker-inside file writer.
func (w *fileLogWriter) startLogger() error {
	file, err := w.createLogFile()
	if err != nil {
		return err
	}
	if w.fileWriter != nil {
		w.fileWriter.Close()
	}
	w.fileWriter = file
	return w.initFd()
}

func (w *fileLogWriter) needRotateDaily(size int, day int) bool {
	return (w.MaxLines > 0 && w.maxLinesCurLines >= w.MaxLines) ||
		(w.MaxSize > 0 && w.maxSizeCurSize + size >= w.MaxSize) ||
		(w.Daily && day != w.dailyOpenDate)
}

func (w *fileLogWriter) needRotateHourly(size int, hour int) bool {
	return (w.MaxLines > 0 && w.maxLinesCurLines >= w.MaxLines) ||
		(w.MaxSize > 0 && w.maxSizeCurSize + size >= w.MaxSize) ||
		(w.Hourly && hour != w.hourlyOpenDate)

}

// 写入消息, 会导致文件切换
func (w *fileLogWriter) WriteMsg(when time.Time, msg string, level int) error {
	if level > w.Level {
		return nil
	}
	hd, d, h := formatTimeHeader(when)
	msg = string(hd) + msg + "\n"
	if w.Rotate {
		w.RLock()  // 双锁机制, 可重入锁判断是否需要切换; 不可重入锁进行文件切换
		if w.needRotateHourly(len(msg), h) {
			w.RUnlock()
			w.Lock()
			if w.needRotateHourly(len(msg), h) { // 两次判断,增加可靠性
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

	w.Lock()
	_, err := w.fileWriter.Write([]byte(msg))
	if err == nil {
		w.maxLinesCurLines++
		w.maxSizeCurSize += len(msg)
	}
	w.Unlock()
	return err
}

// 创建log文件, 并且调整其权限为设置的权限
func (w *fileLogWriter) createLogFile() (*os.File, error) {
	// Open the log file
	perm, err := strconv.ParseInt(w.Perm, 8, 64)
	if err != nil {
		return nil, err
	}

	filepath := path.Dir(w.Filename)
	os.MkdirAll(filepath, os.FileMode(perm))

	fd, err := os.OpenFile(w.Filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, os.FileMode(perm))
	if err == nil {
		// Make sure file perm is user set perm cause of `os.OpenFile` will obey umask
		os.Chmod(w.Filename, os.FileMode(perm))
	}
	return fd, err
}

// 初始化私有变量, 并且开启 goroutine 进行异步处理消息
func (w *fileLogWriter) initFd() error {
	fd := w.fileWriter
	fInfo, err := fd.Stat()
	if err != nil {
		return fmt.Errorf("get stat err: %s", err)
	}
	w.maxSizeCurSize = int(fInfo.Size())
	w.dailyOpenTime = time.Now()
	w.dailyOpenDate = w.dailyOpenTime.Day()
	w.hourlyOpenTime = time.Now()
	w.hourlyOpenDate = w.hourlyOpenTime.Hour()
	w.maxLinesCurLines = 0
	if w.Hourly {
		go w.hourlyRotate(w.hourlyOpenTime)
	} else if w.Daily {
		go w.dailyRotate(w.dailyOpenTime)
	}
	if fInfo.Size() > 0 && w.MaxLines > 0 {
		count, err := w.lines()
		if err != nil {
			return err
		}
		w.maxLinesCurLines = count
	}
	return nil
}

// 按照天数轮询. 在开启的goroutine是一个定时器, 定时进行文件切换的相关操作
func (w *fileLogWriter) dailyRotate(openTime time.Time) {
	y, m, d := openTime.Add(24 * time.Hour).Date()
	nextDay := time.Date(y, m, d, 0, 0, 0, 0, openTime.Location())
	tm := time.NewTimer(time.Duration(nextDay.UnixNano() - openTime.UnixNano() + 100)) //定时器
	<-tm.C // 阻塞等待,直到新的一轮的开始
	w.Lock()
	if w.needRotateDaily(0, time.Now().Day()) {
		if err := w.doRotate(time.Now()); err != nil {
			fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.Filename, err)
		}
	}
	w.Unlock()
}

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

func (w *fileLogWriter) lines() (int, error) {
	fd, err := os.Open(w.Filename)
	if err != nil {
		return 0, err
	}
	defer fd.Close()

	buf := make([]byte, 32768) // 32k
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := fd.Read(buf)
		if err != nil && err != io.EOF {
			return count, err
		}

		count += bytes.Count(buf[:c], lineSep)

		if err == io.EOF {
			break
		}
	}

	return count, nil
}

// 切换日志文件操作
// new file name like xx.2013-01-01.log (daily) or xx.001.log (by line or size)
func (w *fileLogWriter) doRotate(logTime time.Time) error {
	// file exists
	// Find the next available number
	num := w.MaxFilesCurFiles + 1
	fName := ""
	format := ""
	var openTime time.Time
	rotatePerm, err := strconv.ParseInt(w.RotatePerm, 8, 64)
	if err != nil {
		return err
	}

	_, err = os.Lstat(w.Filename)
	if err != nil {
		// 文件不存在或者其它状况
		goto RESTART_LOGGER
	}

	if w.Hourly {
		format = "2006010215"
		openTime = w.hourlyOpenTime
	} else if w.Daily {
		format = "2006-01-02"
		openTime = w.dailyOpenTime
	}

	// 设置了单个文件限制的MaxLines, MaxSize, 通过遍历设置文件的新名称
	if w.MaxLines > 0 || w.MaxSize > 0 {
		for ; err == nil && num <= w.MaxFiles; num++ {
			fName = w.fileNameOnly + fmt.Sprintf(".%s.%03d%s", logTime.Format(format), num, w.suffix)
			_, err = os.Lstat(fName)
		}
	} else {
		fName = w.fileNameOnly + fmt.Sprintf(".%s.%03d%s", openTime.Format(format), num, w.suffix)
		_, err = os.Lstat(fName)
		w.MaxFilesCurFiles = num
	}

	// return error if the last file checked still existed
	if err == nil {
		return fmt.Errorf("Rotate: Cannot find free log number to rename %s", w.Filename)
	}

	// 关闭之前的文件
	w.fileWriter.Close()

	// 文件重命名,切换操作,系统调用
	err = os.Rename(w.Filename, fName)
	if err != nil {
		goto RESTART_LOGGER
	}

	err = os.Chmod(fName, os.FileMode(rotatePerm)) // 修改历史的文件的权限

	//在命名失败 或者 文件被删除, 正常执行到此都会执行
RESTART_LOGGER:

	startLoggerErr := w.startLogger() // 重新启动log
	go w.deleteOldLog() //删除旧文件

	if startLoggerErr != nil {
		return fmt.Errorf("Rotate StartLogger: %s", startLoggerErr)
	}
	if err != nil {
		return fmt.Errorf("Rotate: %s", err)
	}
	return nil
}

// 删除文件(??) 问题是最大文件数目没有作用?
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

// Destroy close the file description, close file writer.
func (w *fileLogWriter) Destroy() {
	w.fileWriter.Close()
}

// Flush flush file logger.
// there are no buffering messages in file logger in memory.
// flush file means sync file from disk.
func (w *fileLogWriter) Flush() {
	w.fileWriter.Sync()
}

func init() {
	Register(AdapterFile, newFileWriter)
}

