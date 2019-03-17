# IPC(Inter Process Communication) 进程间通信

- **竞争条件**

多个进程(线程)通过共享内存(或者共享文件)的方式进行通信会出现竞争条件. 竞争条件, 通俗的说, 就是
两个或者多个进程读写某个共享数据, 而最后的结果取决于进程运行的精确时序.

- **临界区**

程序可以分为两个部分: 不会导致竞争条件的程序片段和会导致竞争条件的程序片段. 会导致竞争条件的程序
片段称为临界区. 避免竞争条件只需要阻止多个进程同时读写共享数据就可以了, 也就是保证同时只有一个进
程处于临界区内.

![avatar](../resource/concurrent-zone.png)


## 锁

**锁就是保证只有一个进程处于临界区一种机制.**

- **硬件**

对于单处理器而言, 临界区问题的解决方案: 修改共享变量时禁止中断出现. 但是屏蔽中断后, 时钟中断也会被屏蔽.
相当于这个时候我们把CPU交给了这个进程, 它不会因为CPU时钟走完而切换, 如果其不在打开中断, 后果非常可怕. 
总之, 把屏蔽中断的权利交给用户级进程很不明智.

对于多处理器而言, 上述的操作是无效的.

现代计算机系统提供了特殊硬件指令以允许能**`原子地(不可中断地)`**检查和修改变量的内容或交换两个变量的内容(
比如CAS, compare and swap). 锁的软件方案也是通过`原子操作`来实现的.


- **软件**

1.信号量

信号量, 它使用一个整型变量来累计唤醒次数. 一个信号量的取值是0(表示没有保存下来的唤醒次数)或者正值(表示一个
或多个唤醒操作). 信号量除了初始化外只能通过两个标准原子操作: wait() 和 signal()来访问.

2.互斥量

互斥量, 可以认为是取值只能是0和1的信号量


**实现**

每个信号量关联一个等待进程链表. wait()的时候发现信号量为不为正时, 可以选择忙等待, 也可以选择阻塞自己(如下),
进程加入等待链表. signal()事从等待链表中读取进程唤醒.

```cgo
typedef struct {
    int value;
    struct process *list;
} semaphore;
 
wait(semaphore *S) {
    S->value--;
    if (S->value < 0) {
        add this process to S->list;
        block()
    }
}
 
signal(semphore *S) {
    S->value++;
    if (S->value <=0 ) {
        remove a process P from S->list;
        wakeup(P)
    }
}
```

3.锁

**互斥锁**  只有取得互斥锁的进程才能 进入临界区, 无论读写.

**自旋锁**  自旋锁是指在进程师徒获得锁失败的时候选择忙等待而不是阻塞自己. 选择忙等待的优点在于如果该进程在其自身的
CPU时间片内拿到锁(说明占用时间都比较短), 则相比阻塞少了上下文切换. 这里的一个隐藏条件: 多处理器. 因为单处理器的情
况下, 由于当前自旋进程占用着CPU, 持有锁的进程只能等待自旋进程耗尽CPU时间才会有机会执行, 这样CPU就空转了.

**读写锁**  读写锁要根据进程进入临界区的具体行为(读,写)来决定锁的占用情况. 这样锁的状态有三种: 读模式加锁, 写模
式加锁, 无锁.


**忙等待 vs 阻塞**

忙等待会使CPU空转, 好处是如果CPU在当前时间片内锁被其他进程释放,当前进程直接就能拿到锁而不需要CPU进行进程调度了. 适
用于锁占用时间较短的状况, 且不适合单处理器.

阻塞不会导致CPU空转, 但是进程切换也需要代价, 比如上下文切换, CPU Cache Miss.


## golang当中锁的实现

```cgo
type Mutex struct {
    state int32
    sema  uint32
}
 
const (
    mutexLocked = 1 << iota // 1 << 0
    mutexWoken              // 1 << 1
    mutexWaiterShift = iota // 2
)
```

golang当中互斥锁是使用信号量的方式实现的. 
sema就是信号量, 一个非负数;
state表示Mutex的状态. **mutexLocked**表示锁是否可用(0可用, 1被别的goroutine占用), **mutexWoken=2**表示mutex是否被
唤醒, **mutexWaiterShift=2**表示统计阻塞在该mutex上goroutine数目需要移位的数值.

将上述的三个常量映射到state上就是:

```
state:   |32|31|...|3|2|1|
         \__________/ | |
               |      | |
               |      | mutex的占用状态（1被占用，0可用）
               |      |
               |      mutex的当前goroutine是否被唤醒
               |
               当前阻塞在mutex上的goroutine数
```

Lock() 方法:

```cgo
func (m *Mutex) Lock() {
    // CAS 原子操作, 尝试获得锁, 如果m.state == 0, 则将其设置为1, 并返回
    if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
        // go做race检测时候使用, 需要带上-race, 则race.Enable被设置位True
        if race.Enabled {
            race.Acquire(unsafe.Pointer(m))
        }
        return
    }
    
    awoke := false
    iter := 0
    for {
        old := m.state // 当前值
        new := old | mutexLocked // old+1 或者 old, new都是被占用
        
        // 表示当前的mutex被占用
        if old&mutexLocked != 0 {
            // 锁的自旋版本. golang的自旋锁做了一些取舍限制: 1.多核; 2.GOMAXPROCES>1;
            // 3. 至少有一个运行的P并且local的P队列为空. 公狼的自旋尝试只会做几次, 并不会
            // 一直尝试下去.
            if runtime_canSpin(iter) {
                // Active spinning makes sense.
                // Try to set mutexWoken flag to inform Unlock
                // to not wake other blocked goroutines.
                if !awoke && old&mutexWoken == 0 && old>>mutexWaiterShift != 0 &&
                    atomic.CompareAndSwapInt32(&m.state, old, old|mutexWoken) {
                    awoke = true
                }
                runtime_doSpin() 
                iter++
                continue
            }
            
            new = old + 1<<mutexWaiterShift // 表示mutex的等待goroutine的数量加1
        }
        
        if awoke {
            // awoke是True,表示goroutine已经被唤醒
            if new&mutexWoken == 0 {
                panic("sync: inconsistent mutex state")
            }
            // new = new & ^mutexWoken => new = new & 1, 只是保留了new是否被占用位
            new &^= mutexWoken // goroutine已经被唤醒, 需要将 m.state 的标志位去掉
        }
        
        // 试图将m.state设置为new
        if atomic.CompareAndSwapInt32(&m.state, old, new) {
            // 如果m.state之前的值(old) 没有被占用, 则表示当前goroutine拿到了锁
            if old&mutexLocked == 0 {
                break
            }
            
            // 信号量的wait()操作, 如果 m.sema < 0,则当前goroutine塞入信号量 m.sema关联的
            // goroutine waiting list, 并且休眠.
            runtime_Semacquire(&m.sema)
            awoke = true
            iter = 0
        }
    }

    if race.Enabled {
        race.Acquire(unsafe.Pointer(m))
    }
}
```


```cgo
func (m *Mutex) Unlock() {
    if race.Enabled {
        _ = m.state
        race.Release(unsafe.Pointer(m))
    }
 
    // 将m.state的锁置为可用
    new := atomic.AddInt32(&m.state, -mutexLocked)
    if (new+mutexLocked)&mutexLocked == 0 {
        panic("sync: unlock of unlocked mutex")
    }
 
    old := new
    for {
        // 如果阻塞在该锁上的goroutine数目为0或者Mutex处于lock或者唤醒状态, 
        // 不需要唤醒任何goroutine,直接返回
        if old>>mutexWaiterShift == 0 || old&(mutexLocked|mutexWoken) != 0 {
            return
        }
        // 先将阻塞在mutex上的goroutine数目减1, 然后将mutex设置为唤醒状态
        new = (old - 1<<mutexWaiterShift) | mutexWoken
        if atomic.CompareAndSwapInt32(&m.state, old, new) {
            // runtime_Semrelease和runtime_Semacquire的作用刚好相反, 将阻塞在信号量上goroutine唤醒.
            runtime_Semrelease(&m.sema)
            return
        }
        old = m.state
    }
}
```