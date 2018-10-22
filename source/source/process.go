package source

import (
	"fmt"
)

func main() {
	fmt.Println("Hello, World")
}

/**
编译: 使用-gcflags "-N -l"参数关闭编译器代码优化和函数内联

	go build  -gcflags "-N -l" -o test test.go

启动过程:
	runtime/rt0_linux_amd64.s
		|
	runtime/asm_amd64.s
		|
	// 调用系统初始化函数
	CALL	runtime·args(SB)    -> runtime/runtime1.go  60 (命令行参数)
	CALL	runtime·osinit(SB)  -> runtime/os_linux.go  269 (确定cpu core数量)
	CALL	runtime·schedinit(SB) -> runtime/proc.go    468
		|
	// 创建 main goroutine 用于执行 runtime.main
	CALL	runtime·newproc(SB)  -> runtime/proc.go   2929
	    |
	// 当前线程开始执行 main goroutine
	CALL	runtime·mstart(SB) -> runtime/proc.go  1135
		|
	// golang 代码
	DATA	runtime·mainPC+0(SB)/8,$runtime·main(SB) -> runtime/proc.go  28

*/

/*
 The bootstrap sequence is:
	call osinit
	call schedinit
	make & queue new G
	call runtime·mstart

 func schedinit() {
	// getg 返回指向当前g的指针. 编译器将对此函数的调用重写为指令,直接获取g(来自TLS或专用寄存器)
	_g_ := getg()

	// 最大系统线程数量限制
	sched.maxmcount = 10000

	// 栈, 内存分配器, 调度器相关初始化
	tracebackinit()    // Go变量初始化在运行时启动期间发生. 此函数负责Go变量初始化
	moduledataverify()
	stackinit()
	mallocinit()
	mcommoninit(_g_.m)
	alginit()       // maps must not be used before this call
	modulesinit()   // provides activeModules
	typelinksinit() // uses maps, activeModules
	itabsinit()     // uses activeModules

	msigsave(_g_.m)
	initSigmask = _g_.m.sigmask

	// 处理命令行参数和环境变量
	goargs()
	goenvs()

	// 处理GODEBUG, GOTRACEBACK 调试相关的环境变量设置
	parsedebugvars()

    // 垃圾回收器初始化
	gcinit()

	// 通过CPU Core 和 GOMAXPROCS 环境变量确定P的数量
	sched.lastpoll = uint64(nanotime())
	procs := ncpu
	if n, ok := atoi32(gogetenv("GOMAXPROCS")); ok && n > 0 {
		procs = n
	}
	if procs > _MaxGomaxprocs {
		procs = _MaxGomaxprocs
	}

    // 调整P数量
	if procresize(procs) != nil {
		throw("unknown runnable goroutine during bootstrap")
	}

	if buildVersion == "" {
		// Condition should never trigger. This code just serves
		// to ensure runtime·buildVersion is kept in the resulting binary.
		buildVersion = "unknown"
	}
 }
*/
