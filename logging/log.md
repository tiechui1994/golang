#### beego log实现机制

logging 采用的是日志采集和日志引擎解耦的实现方式. 用户可以自定义log的处理的方式. 官方已经实现了日志的采集, 用户可以自定义日志引擎来处理日志.

日志处理的过程:

```
日志采集(调用log的write方法) -> 日志分发 -> 日志处理(日志引擎进行对日志的处理)
```

日志采集可以分为同步和异步两种方式. 默认启动的是同步方式.

```
// Logger接口: log的顶层设计, 自定义日志必须实现该接口
type Logger interface {
	Init(config string) error
	WriteMsg(when time.Time, msg string, level int) error
	Destroy()
	Flush()
}
```

```
// Beego日志采集相关的数据结构
// nameLogger, 一种日志引擎对于一个特定的名称. 
type nameLogger struct {
	Logger
	name string 
}

// Log消息定义
type logMsg struct {
	level int
	msg   string
	when  time.Time
}

// 日志采集
type BeeLogger struct {
	lock                sync.Mutex
	level               int
	init                bool
	enableFuncCallDepth bool   //是否输出调用函数和行号, 默认是false
	loggerFuncCallDepth int    //记录调用log函数的深度, 默认是2
	asynchronous        bool
	prefix              string  //自定义日志的前缀(标示作用)
	msgChanLen          int64   // 消息队列的大小, 默认1000
	msgChan             chan *logMsg // 消息队列
	signalChan          chan string  // 发送信号让日志Flush,Destory
	wg                  sync.WaitGroup
	outputs             []*nameLogger // 日志引擎集合, 默认只加入了console处理方式
}
```

下面说一下日志启动流程:

```
1. 日志采集调用Register()注册日志的实现方式
2. NewLogger(), 创建一个日志对象. 默认会将console引擎添加到outputs,然后初始化 
3. 调用SetLogger(), 增加自定义的日志, 并初始化. // 至此, 同步日志处理就可以正常使用了.
4. 调用Async(), 启动异步处理日志的方式 // 异步日志处理
```

#### 细节内容

创建log

```go
func NewLogger(channelLens ...int64) *BeeLogger {
	bl := new(BeeLogger)
	bl.level = LevelDebug
	bl.loggerFuncCallDepth = 2 // 设置函数调用的深度
	bl.msgChanLen = append(channelLens, 0)[0]  // 设置msgChanLen, 必须是一个正整数
	if bl.msgChanLen <= 0 {
		bl.msgChanLen = defaultAsyncMsgLen  // 默认值1000 
	}
	bl.signalChan = make(chan string, 1) // flush, close信号,优雅关闭日志
	bl.setLogger(AdapterConsole)  // 添加console的处理方式
	return bl
}
```

```go
func (bl *BeeLogger) setLogger(adapterName string, configs ...string) error {
	config := append(configs, "{}")[0]
    log, ok := adapters[adapterName] // adapters是一个map[string] func() Logger, 在调用对应的引擎名称的方法时候,产生一个引擎实例
	lg := log() //创建一个引擎实例
	err := lg.Init(config) // 初始化
	bl.outputs = append(bl.outputs, &nameLogger{name: adapterName, Logger: lg}) //添加
	return nil
}
```

启动异步log

```go
func (bl *BeeLogger) Async(msgLen ...int64) *BeeLogger {
	if bl.asynchronous { // 异步启动只会被调用一次
		return bl
	}
	bl.asynchronous = true
	if len(msgLen) > 0 && msgLen[0] > 0 { // 可能会重新设置channel的长度
		bl.msgChanLen = msgLen[0]
	}
	bl.msgChan = make(chan *logMsg, bl.msgChanLen) //创建channel
	logMsgPool = &sync.Pool{ // 对象池, 减少GC, 有兴趣可以了解一下
		New: func() interface{} {
			return &logMsg{}
		},
	}
	bl.wg.Add(1)
	go bl.startLogger()  // 新的gorotine当中运行采集工作, 异步启动完成
	return bl
}
```

启动详情过程:

```go
func (bl *BeeLogger) startLogger() { 
	gameOver := false
	for { // 不断循环收集日志
		select {
		case bm := <-bl.msgChan: // --开始会阻塞--
			bl.writeToLoggers(bm.when, bm.msg, bm.level) // 分发处理
			logMsgPool.Put(bm) // 回收对象
		case sg := <-bl.signalChan: //刷新或者优雅关闭日志
			bl.flush()
			if sg == "close" {
				for _, l := range bl.outputs {
					l.Destroy()
				}
				bl.outputs = nil
				gameOver = true
			}
			bl.wg.Done()
		}
		if gameOver {
			break
		}
	}
}
```

```go
func (bl *BeeLogger) writeToLoggers(when time.Time, msg string, level int) {
	for _, l := range bl.outputs {
		err := l.WriteMsg(when, msg, level) //分发处理, 调用Logger实例WriteMsg
	}
}
```



日志收集

```go
// 收集LevelEmergency基本的日志
func (bl *BeeLogger) Emergency(format string, v ...interface{}) {
	if LevelEmergency > bl.level {
		return
	}
	bl.writeMsg(LevelEmergency, format, v...)
}
```

```go
func (bl *BeeLogger) writeMsg(logLevel int, msg string, v ...interface{}) error {
	msg = bl.prefix + " " + msg // 增加日志自定义前缀
	when := time.Now()
	if bl.enableFuncCallDepth {
		_, file, line, ok := runtime.Caller(bl.loggerFuncCallDepth) // 获取调用的函数位置信息
		_, filename := path.Split(file)
		msg = "[" + filename + ":" + strconv.Itoa(line) + "] " + msg
	}

	msg = levelPrefix[logLevel] + msg // 增加系统设置的前缀 "A", "E"等

	// 异步: 异步写入  同步:直接写
	if bl.asynchronous {
		lm := logMsgPool.Get().(*logMsg) // 获取一个记录日志的对象
		lm.level = logLevel
		lm.msg = msg
		lm.when = when
		bl.msgChan <- lm // 异步方式, 先发送到管道, 再由管道进行分发
	} else {
		bl.writeToLoggers(when, msg, logLevel) //同步方式
	}
}
```

关闭日志:

```go
func (bl *BeeLogger) Close() {
	if bl.asynchronous { //异步方式, 发送信号进行通知
		bl.signalChan <- "close" 
		bl.wg.Wait()
		close(bl.msgChan)
	} else {
		bl.flush() // 同步方式, 直接关闭
		for _, l := range bl.outputs {
			l.Destroy()
		}
		bl.outputs = nil
	}
	close(bl.signalChan)
}
```

总结: beego的log处理使用了 `发布/订阅模式`, 将日志的收集和日志的处理分开, 从而达到解耦合的目的.