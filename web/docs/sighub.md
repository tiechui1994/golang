进程组:(PGID)
	一系列相互关联的进程集合, 系统中的每一个进程必须从属于某一个进程组; 每个进程组都会有一个唯一的
	ID, 即PGID
	
	进程组首进程: 首进程ID = 进程组ID


会话:(SID)
	若干进程组的集合, 系统中的每一个进程组必须从属于某个会话;一个会话只拥有最多一个控制终端, 该终端
	为会话中所有进程组中的进程所共用. 
	
	一个会话中前台进程组只有一个, 只有其中的进程才可以和控制终端进行交互; 除了前台进程组外的进程组,都
	是后台进程组;
	
	会话首进程: 首进程ID = 会话ID

关系:
	session = 一个前台进程组 + 多个后台进程组
	进程组 = 多个进程

	一个session可能会有一个session首进程(也可能没有), 而一个session首进程可能会有一个控制终端(也可能没有).
	一个进程组可能会有一个进程组首进程(也可能没有).


SIGHUP信号的触发及其默认处理:
	系统对SIGHUB信号的默认处理是终止收到该信号的进程. 所以,若程序中没有捕捉该信号, 当收到该信号时, 进程就会退出.
	
	SIGHUB触发情况:
		1. 终端关闭, 该信号被发送到当前session中的所有进程.
		2. session首进程退出(kill命令), 该信号被发送到该session中前台进程组的每一个进程;
		3. 若父进程退出导致进程组成为孤儿进程组, 且该进程组中有进程处于停止状态(收到SIGSTOP或SIGSTP),
		该信号会被发送到该进程组中的每一个进程.

测试:

```c
#include <stdio.h>
#include <signal.h>
#include <unistd.h>

char **args;

void exithandle(int sig) {
    printf("%s : sighup received ", args[1]);
}

int main(int argc, char **argv) {
    args = argv;
    signal(SIGHUP, exithandle);
    pause();
    return 0;
}
```

将上述代码编译成signal

```shell
#!/bin/sh
signal &
```

上述的脚本是sig.sh

```
	1. 执行命令: signal front > t.txt 前台进程 (或者 signal backend > t.txt 后台进程);
	   关闭终端; 在t.txt当中都会存在内容. 验证情况1
	2. 执行命令: sh sig.sh;
		 关闭终端; 通过ps -ef|grep signal可以看到该进程还在, 但是t.txt为空.  验证情况3.
		 解释: 执行sh sig.sh时, 会启动一个进程(signal), 然后shell退出, 导致signal属于孤儿进程组,
		 在session首进程退出后, 由于signal还处于运行状态, 所以不会收到SIGHUB信号.
```

```shell
	终端关闭,进程不退出:
	方式一:
		trap "" SIGHUP # 屏蔽SIGHUP信号
		signal

	方式二:
		shell中执行 signal & # 孤儿进程
```





























