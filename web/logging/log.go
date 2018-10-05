package logging

import (
	"strings"
	"time"
	"sync"
	"os"
	"runtime"
	"path"
	"strconv"
	"fmt"
	"log"
)

//import "github.com/astaxie/beego/logs"
//
//	log := NewLogger(10000)
//	log.SetLogger("console", "")
//
//	> the first params stand for how many channel
//
//Use it like this:
//
//	log.Trace("trace")
//	log.Info("info")
//	log.Warn("warning")
//	log.Debug("debug")
//	log.Critical("critical")
//

const (
	LevelEmergency = iota
	LevelAlert
	LevelCritical
	LevelError
	LevelWarning
	LevelNotice
	LevelInformational
	LevelDebug
)

// 作为参考设置日志级别
const levelLoggerImpl = -1

// 官方实现的Log
const (
	AdapterConsole   = "console"
	AdapterFile      = "file"
	AdapterMultiFile = "multifile"
	AdapterMail      = "smtp"
	AdapterConn      = "conn"
	AdapterEs        = "es"
	AdapterJianLiao  = "jianliao"
	AdapterSlack     = "slack"
	AdapterAliLS     = "alils"
)

// 常见的Log级别 -> 定义的Log级别(实际设计当中可选)
const (
	LevelInfo  = LevelInformational
	LevelTrace = LevelDebug
	LevelWarn  = LevelWarning
)

// 注册函数 -> 初始化调用.
// 扩展性考虑
type newLoggerFunc func() Logger

// Log的顶层接口
type Logger interface {
	Init(config string) error
	WriteMsg(when time.Time, msg string, level int) error
	Destroy()
	Flush()
}

// 保存所有实现的Logger的引擎
var adapters = make(map[string]newLoggerFunc)
var levelPrefix = [LevelDebug + 1]string{"[M] ", "[A] ", "[C] ", "[E] ", "[W] ", "[N] ", "[I] ", "[D] "}

// 注册的方式增加Logger的实现, 主要目的是为了扩展
func Register(name string, log newLoggerFunc) {
	if log == nil {
		panic("logs: Register provide is nil")
	}
	if _, dup := adapters[name]; dup {
		panic("logs: Register called twice for provider " + name)
	}
	adapters[name] = log
}

//--------------------------------------------------------------------------------------------------------------------

// 对Logger进一步封装, 目的是为了每个实现的Logger进行命名, 方便操作
type nameLogger struct {
	Logger
	name string //adapter的名称
}

// Log消息定义, 实际转发的参数封装
type logMsg struct {
	level int
	msg   string
	when  time.Time
}

// Logger的调度器, 管理所有用户添加的Logger
// 注意: 用户添加的Logger != 注册的Logger
type BeeLogger struct {
	lock                sync.Mutex    // 锁定init只能是一次, 锁定启动异步日志, 锁定添加Logger引擎顺序
	level               int           // Logger引擎定义的Log级别(可配置)
	init                bool          // 初始化控制, 只能初始化一次
	enableFuncCallDepth bool          // 是否记录log产生的位置
	loggerFuncCallDepth int           // 当记录log产生的位置, 记录位置的深度
	prefix              string        // Logger引擎自定义独特的标识符(可配置)
	outputs             []*nameLogger // 用户添加的Logger引擎

	asynchronous bool
	wg           sync.WaitGroup
	msgChanLen   int64        // 日志管道长度
	msgChan      chan *logMsg // 消息管道, 进行异步转发消息 -> msgChanLen
	signalChan   chan string  // 信号, 注意负责日志的flush和destroy -> 1
}

//默认异步log的通道的长度
const defaultAsyncMsgLen = 1e3

// 一个sync.Pool对象就是一组临时对象的集合.Pool是协程安全的. Pool用于存储那些被分配了但是没有被使用,而未来可能会使用的值,以减小垃圾回收的压力
var logMsgPool *sync.Pool

// 构建一个Logger调度器
func NewLogger(channelLens ...int64) *BeeLogger {
	bl := new(BeeLogger)
	bl.level = LevelDebug
	bl.loggerFuncCallDepth = 2

	// 只是指定了msgChanLen, 但是并没有实例化 msgChan(实例化了msgChan意味着异步日志)
	bl.msgChanLen = append(channelLens, 0)[0]
	if bl.msgChanLen <= 0 {
		bl.msgChanLen = defaultAsyncMsgLen
	}

	// 控制异步日志的flush与close
	bl.signalChan = make(chan string, 1)
	bl.setLogger(AdapterConsole) // 默认的会增加 AdapterConsole 引擎(控制台日志)
	return bl
}

// 启动异步日志(默认是同步日志), 任何时刻, 同时还可以设置日志队列长度
func (bl *BeeLogger) Async(msgLen ...int64) *BeeLogger {
	bl.lock.Lock()
	defer bl.lock.Unlock()
	if bl.asynchronous {
		return bl
	}
	bl.asynchronous = true
	if len(msgLen) > 0 && msgLen[0] > 0 {
		bl.msgChanLen = msgLen[0]
	}

	// 异步日志的标志
	bl.msgChan = make(chan *logMsg, bl.msgChanLen)

	// logMsg池, 专门产生logMsg对象
	logMsgPool = &sync.Pool{
		New: func() interface{} {
			return &logMsg{}
		},
	}
	bl.wg.Add(1) // 主线程
	go bl.startLogger()
	return bl
}

// 异步日志: 循环从msgChan读取日志并进行分发
func (bl *BeeLogger) startLogger() {
	// 经典: 日志的flush和close
	// flush -> 清空当前的msgChan的所有的日志, 协成空转
	// close -> 清空当前的msgChan的所有的日志, 协成关闭
	gameOver := false
	for {
		select {
		case bm := <-bl.msgChan:
			bl.writeToLoggers(bm.when, bm.msg, bm.level) // 异步分发操作 ^ _ ^
			logMsgPool.Put(bm)
		case sg := <-bl.signalChan:
			// Now should only send "flush" or "close" to bl.signalChan
			bl.flush()
			if sg == "close" {
				for _, l := range bl.outputs {
					l.Destroy()
				}
				bl.outputs = nil
				gameOver = true
			}

			// 这里协程不会退出,会继续空转, 但是主现成Wait()方法阻塞关闭
			bl.wg.Done() // 发送一个信号,表示当前的工作完成
		}

		// 协程会关闭
		if gameOver {
			break
		}
	}
}

// 增加Adapter
func (bl *BeeLogger) SetLogger(adapterName string, configs ...string) error {
	bl.lock.Lock()
	defer bl.lock.Unlock()
	if !bl.init {
		bl.outputs = []*nameLogger{}
		bl.init = true
	}
	return bl.setLogger(adapterName, configs...)
}

// 实例化并初始化一个Adapter, 在这里进行日志引擎的初始化工作
func (bl *BeeLogger) setLogger(adapterName string, configs ...string) error {
	config := append(configs, "{}")[0]
	for _, l := range bl.outputs {
		if l.name == adapterName {
			return fmt.Errorf("logs: duplicate adaptername %q (you have set this logger before)", adapterName)
		}
	}

	log, ok := adapters[adapterName]
	if !ok {
		return fmt.Errorf("logs: unknown adaptername %q (forgotten Register?)", adapterName)
	}

	lg := log()            // 创建一个引擎实例
	err := lg.Init(config) // 初始化
	if err != nil {
		fmt.Fprintln(os.Stderr, "logs.BeeLogger.SetLogger: "+err.Error())
		return err
	}
	bl.outputs = append(bl.outputs, &nameLogger{name: adapterName, Logger: lg})
	return nil
}

// 删除指定日志引擎
func (bl *BeeLogger) DelLogger(adapterName string) error {
	bl.lock.Lock()
	defer bl.lock.Unlock()
	outputs := make([]*nameLogger, 0)
	for _, lg := range bl.outputs {
		if lg.name == adapterName {
			lg.Destroy()
		} else {
			outputs = append(outputs, lg)
		}
	}
	if len(outputs) == len(bl.outputs) {
		return fmt.Errorf("logs: unknown adaptername %q (forgotten Register?)", adapterName)
	}
	bl.outputs = outputs
	return nil
}

// 日志分发操作:
// 		同步分发,
// 		异步分发, flush分发
func (bl *BeeLogger) writeToLoggers(when time.Time, msg string, level int) {
	for _, l := range bl.outputs {
		err := l.WriteMsg(when, msg, level)
		if err != nil {
			fmt.Fprintf(os.Stderr, "unable to WriteMsg to adapter:%v,error:%v\n", l.name, err)
		}
	}
}

// 写入:
func (bl *BeeLogger) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	// 写入日志总是在末尾添加'\n', 因此需要去掉末尾的'\n'
	if p[len(p)-1] == '\n' {
		p = p[0 : len(p)-1]
	}
	// set levelLoggerImpl to ensure all log message will be write out
	err = bl.writeMsg(levelLoggerImpl, string(p))
	if err == nil {
		return len(p), err
	}
	return 0, err
}

// 写入日志:
// 		logLevel是日志级别, msg是日志格式(v是日志对应格式内容)/日志内容
func (bl *BeeLogger) writeMsg(logLevel int, msg string, v ...interface{}) error {
	// 什么情况?
	if !bl.init {
		bl.lock.Lock()
		bl.setLogger(AdapterConsole)
		bl.lock.Unlock()
	}

	if len(v) > 0 {
		msg = fmt.Sprintf(msg, v...)
	}

	msg = bl.prefix + " " + msg // 增加日志自定义前缀

	when := time.Now()
	if bl.enableFuncCallDepth {
		_, file, line, ok := runtime.Caller(bl.loggerFuncCallDepth)
		if !ok {
			file = "???"
			line = 0
		}
		_, filename := path.Split(file)
		msg = "[" + filename + ":" + strconv.Itoa(line) + "] " + msg
	}

	// 当传入日志级别为-1时
	if logLevel == levelLoggerImpl {
		logLevel = LevelEmergency
	} else {
		msg = levelPrefix[logLevel] + msg // 增加系统设置的前缀
	}

	// 日志写入, 同步:分发  异步:管道->分发
	if bl.asynchronous {
		lm := logMsgPool.Get().(*logMsg) // 获取一个记录日志的对象
		lm.level = logLevel
		lm.msg = msg
		lm.when = when
		bl.msgChan <- lm // 发送到日志通道当中
	} else {
		bl.writeToLoggers(when, msg, logLevel) // 同步分发操作
	}
	return nil
}

// SetLevel设置日志消息级别.
// 如果message级别(例如LevelDebug)高于logger级别(例如LevelWarning), 不会写入日志
func (bl *BeeLogger) SetLevel(l int) {
	bl.level = l
}

// 获取message级别.
func (bl *BeeLogger) GetLevel() int {
	return bl.level
}

// SetLogFuncCallDepth set log funcCallDepth
func (bl *BeeLogger) SetLogFuncCallDepth(d int) {
	bl.loggerFuncCallDepth = d
}

// GetLogFuncCallDepth return log funcCallDepth for wrapper
func (bl *BeeLogger) GetLogFuncCallDepth() int {
	return bl.loggerFuncCallDepth
}

// EnableFuncCallDepth enable log funcCallDepth
func (bl *BeeLogger) EnableFuncCallDepth(b bool) {
	bl.enableFuncCallDepth = b
}

// set prefix
func (bl *BeeLogger) SetPrefix(s string) {
	bl.prefix = s
}

// 写日志
// Emergency Log EMERGENCY level message.
func (bl *BeeLogger) Emergency(format string, v ...interface{}) {
	if LevelEmergency > bl.level {
		return
	}
	bl.writeMsg(LevelEmergency, format, v...)
}

// Alert Log ALERT level message.
func (bl *BeeLogger) Alert(format string, v ...interface{}) {
	if LevelAlert > bl.level {
		return
	}
	bl.writeMsg(LevelAlert, format, v...)
}

// Critical Log CRITICAL level message.
func (bl *BeeLogger) Critical(format string, v ...interface{}) {
	if LevelCritical > bl.level {
		return
	}
	bl.writeMsg(LevelCritical, format, v...)
}

// Error Log ERROR level message.
func (bl *BeeLogger) Error(format string, v ...interface{}) {
	if LevelError > bl.level {
		return
	}
	bl.writeMsg(LevelError, format, v...)
}

// Warning Log WARNING level message.
func (bl *BeeLogger) Warning(format string, v ...interface{}) {
	if LevelWarn > bl.level {
		return
	}
	bl.writeMsg(LevelWarn, format, v...)
}

func (bl *BeeLogger) Notice(format string, v ...interface{}) {
	if LevelNotice > bl.level {
		return
	}
	bl.writeMsg(LevelNotice, format, v...)
}

func (bl *BeeLogger) Informational(format string, v ...interface{}) {
	if LevelInfo > bl.level {
		return
	}
	bl.writeMsg(LevelInfo, format, v...)
}

func (bl *BeeLogger) Debug(format string, v ...interface{}) {
	if LevelDebug > bl.level {
		return
	}
	bl.writeMsg(LevelDebug, format, v...)
}

func (bl *BeeLogger) Warn(format string, v ...interface{}) {
	if LevelWarn > bl.level {
		return
	}
	bl.writeMsg(LevelWarn, format, v...)
}

func (bl *BeeLogger) Info(format string, v ...interface{}) {
	if LevelInfo > bl.level {
		return
	}
	bl.writeMsg(LevelInfo, format, v...)
}

func (bl *BeeLogger) Trace(format string, v ...interface{}) {
	if LevelDebug > bl.level {
		return
	}
	bl.writeMsg(LevelDebug, format, v...)
}

// 刷新
func (bl *BeeLogger) Flush() {
	if bl.asynchronous {
		bl.signalChan <- "flush"
		bl.wg.Wait()
		bl.wg.Add(1)
		return
	}
	// 同步: 刷新
	bl.flush()
}

// 关闭
func (bl *BeeLogger) Close() {
	if bl.asynchronous {
		bl.signalChan <- "close"
		bl.wg.Wait()
		close(bl.msgChan)
	} else {
		// 同步: 刷新 + 销毁
		bl.flush()
		for _, l := range bl.outputs {
			l.Destroy()
		}
		bl.outputs = nil
	}
	close(bl.signalChan)
}

// 重置: 销毁所有日志引擎, 依旧还可以添加新的日志引擎
func (bl *BeeLogger) Reset() {
	bl.Flush()
	for _, l := range bl.outputs {
		l.Destroy()
	}

	// 将outputs置空, 在这里等价于: bl.outputs = []*nameLogger{}, 地址不会发生改变
	bl.outputs = nil
}

// 日志flush操作
func (bl *BeeLogger) flush() {
	if bl.asynchronous {
		for {
			if len(bl.msgChan) > 0 {
				bm := <-bl.msgChan
				bl.writeToLoggers(bm.when, bm.msg, bm.level) // flush操作
				logMsgPool.Put(bm)                           //回收对象
				continue
			}
			break
		}
	}
	for _, l := range bl.outputs {
		l.Flush()
	}
}

//--------------------------------------------------------------------------------------------------------------------

// 初始化一个日志对象
var beeLogger = NewLogger()

// GetBeeLogger returns the default BeeLogger
func GetBeeLogger() *BeeLogger {
	return beeLogger
}

var beeLoggerMap = struct {
	sync.RWMutex
	logs map[string]*log.Logger
}{
	logs: map[string]*log.Logger{},
}

// GetLogger returns the default BeeLogger
func GetLogger(prefixes ...string) *log.Logger {
	prefix := append(prefixes, "")[0]
	if prefix != "" {
		prefix = fmt.Sprintf(`[%s] `, strings.ToUpper(prefix))
	}
	beeLoggerMap.RLock()
	l, ok := beeLoggerMap.logs[prefix]
	if ok {
		beeLoggerMap.RUnlock()
		return l
	}
	beeLoggerMap.RUnlock()
	beeLoggerMap.Lock()
	defer beeLoggerMap.Unlock()
	l, ok = beeLoggerMap.logs[prefix]
	if !ok {
		l = log.New(beeLogger, prefix, 0)
		beeLoggerMap.logs[prefix] = l
	}
	return l
}

// Reset will remove all the adapter
func Reset() {
	beeLogger.Reset()
}

// Async set the beelogger with Async mode and hold msglen messages
func Async(msgLen ...int64) *BeeLogger {
	return beeLogger.Async(msgLen...)
}

// SetLevel sets the global log level used by the simple logger.
func SetLevel(l int) {
	beeLogger.SetLevel(l)
}

// SetPrefix sets the prefix
func SetPrefix(s string) {
	beeLogger.SetPrefix(s)
}

// EnableFuncCallDepth enable log funcCallDepth
func EnableFuncCallDepth(b bool) {
	beeLogger.enableFuncCallDepth = b
}

// SetLogFuncCall set the CallDepth, default is 4
func SetLogFuncCall(b bool) {
	beeLogger.EnableFuncCallDepth(b)
	beeLogger.SetLogFuncCallDepth(4)
}

// SetLogFuncCallDepth set log funcCallDepth
func SetLogFuncCallDepth(d int) {
	beeLogger.loggerFuncCallDepth = d
}

// SetLogger sets a new logger.
func SetLogger(adapter string, config ...string) error {
	return beeLogger.SetLogger(adapter, config...)
}

// Emergency logs a message at emergency level.
func Emergency(f interface{}, v ...interface{}) {
	beeLogger.Emergency(formatLog(f, v...))
}

// Alert logs a message at alert level.
func Alert(f interface{}, v ...interface{}) {
	beeLogger.Alert(formatLog(f, v...))
}

// Critical logs a message at critical level.
func Critical(f interface{}, v ...interface{}) {
	beeLogger.Critical(formatLog(f, v...))
}

// Error logs a message at error level.
func Error(f interface{}, v ...interface{}) {
	beeLogger.Error(formatLog(f, v...))
}

// Warning logs a message at warning level.
func Warning(f interface{}, v ...interface{}) {
	beeLogger.Warn(formatLog(f, v...))
}

// Warn compatibility alias for Warning()
func Warn(f interface{}, v ...interface{}) {
	beeLogger.Warn(formatLog(f, v...))
}

// Notice logs a message at notice level.
func Notice(f interface{}, v ...interface{}) {
	beeLogger.Notice(formatLog(f, v...))
}

// Informational logs a message at info level.
func Informational(f interface{}, v ...interface{}) {
	beeLogger.Info(formatLog(f, v...))
}

// Info compatibility alias for Warning()
func Info(f interface{}, v ...interface{}) {
	beeLogger.Info(formatLog(f, v...))
}

// Debug logs a message at debug level.
func Debug(f interface{}, v ...interface{}) {
	beeLogger.Debug(formatLog(f, v...))
}

// Trace logs a message at trace level.
// compatibility alias for Warning()
func Trace(f interface{}, v ...interface{}) {
	beeLogger.Trace(formatLog(f, v...))
}

func formatLog(f interface{}, v ...interface{}) string {
	var msg string
	switch f.(type) {
	case string:
		msg = f.(string)
		if len(v) == 0 {
			return msg
		}
		if strings.Contains(msg, "%") && !strings.Contains(msg, "%%") {
			//format string
		} else {
			//do not contain format char
			msg += strings.Repeat(" %v", len(v))
		}
	default:
		msg = fmt.Sprint(f)
		if len(v) == 0 {
			return msg
		}
		msg += strings.Repeat(" %v", len(v))
	}
	return fmt.Sprintf(msg, v...)
}
