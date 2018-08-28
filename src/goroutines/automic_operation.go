package goroutines

import (
	"sync"
	"fmt"
	"time"
)

/*
原子操作:
	原子操作是编程中"最小的且不可并行化"的操作.

	一般情况下,原子操作都是通过"互斥"房屋来保证访问的, 通常由特殊的CPU指令提供保护.
*/

var total struct {
	sync.Mutex
	value int
}

func Work(wg *sync.WaitGroup) {
	defer wg.Done()

	for i := 0; i < 100; i++ {
		total.Lock()
		total.value += 1
		fmt.Println(total.value)
		total.Unlock()
	}
}

func Main() {
	var wg sync.WaitGroup
	wg.Add(2)

	go Work(&wg)
	go Work(&wg)

	wg.Wait()
	fmt.Println(total.value)
}

/*
sync/atomic包
	对于基本的数值类型及复杂对象的读写都提供了原子操作的支持. atomic.Value 原子对象提供了Load
	和store两个原子方法. 分别用于加载和保存数据, 返回值和参数都是interface{}类型, 因此可以用
	于任意的自定义复杂类型.

顺序一致性内存模型:
	如果要保证线程之间数据同步, 原子操作已经为编程人员提供了一些同步保障. 不过这种保障有有一个前提:
	顺序一致性的内存模型.

	在Go当中, 同一个goroutine线程内部, 顺序一致性内存模型的得到保证的. 但是不同的goroutine之间,
	并不满足顺序一致性内存模型, 需要通过明确定义的同步事件来作为同步的参考.
*/

// 此程序无法确定输出结果
func Run() {
	go println("Hello World")
}

/*
在go当中, main函数退出时程序结束, 不会等等任何后台线程. 因为goroutine的执行和main函数的返回事件
是并发的, 对于Run方法打印的结果是未知的.

解决上述问题的方法: 通过同步原语来给两个事件明确排序
	1. channel, channel是阻塞执行.
	2. Mutex, Unlock 是在 Lock之后执行的
*/

func ChannelRun() {
	done := make(chan int)

	go func() {
		println("Hello World")
		done <- 1
	}()

	<-done
}

func MutexRun() {
	var mutex sync.Mutex

	mutex.Lock()

	go func() {
		println("Hello World")
		mutex.Unlock()
	}()

	mutex.Lock()
}

/*
Channel通信:
	对于从无缓冲信道进行的接收, 发生在对该信道进行的发送完成之前.
	对于带缓冲的Channel, 对于Channel的第K个接收完成操作发生在K+C个发送操作完成之前, 其中C是Channel
	的缓存大小.
*/

/*
常见的并发模式:
	并发编程的核心概念是同步通信, 同步通信的方式有很多种.

	1. sync.Mutex
	func main() {
		var mu sync.Mutex

		mu.Lock()
		go func(){
			printf("Hello world")
			mu.Unlock()
		}()
		mu.Lock()
	}

	说明: 两次互斥锁,必然导致第二次申请锁阻塞


	2. 管道
	func main(){
		done := make(chan int)

		go func(){
			printf("Hello world")
			done <- 1
		}()
		<- done
	}

	说明: 对于无缓冲的信道进行的接收, 发生在对该信道进行的发送完成之前.

	func main() {
		done := make(chan int, 10)
		for i := 0; i < cap(done); i++ {
			go func() {
				print("Hello world")
				done <- 1
			}()
		}

		for i := 0; i < cap(done); i++ {
			<-done
		}
	}

	func main() {
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)  // 用于增加等待事件的个数, 必选确保在后台线程启动之前执行
			go func() {
				print("Hello world")
				wg.Done() // 表示完成一个事件
			}()
		}

		wg.Wait() // 等待全部事件完成
	}


	3. 生产者-消费者模型
	func Producer(factor int, out chan<- int) {
		for i:=0; ;i++ {
			// ....
			out <- i*factor
		}
	}

	func Consumer(in <-chan int) {
		for v := range in {
		  // ...
		}
	}

	4. 发布/订阅模型(Publisher, Topic, Subscriber)
	1) 发布者: 维持一个map, key是Subscriber, value是Topic
	2) 订阅者:
*/

type (
	subscriber chan interface{}        // 订阅在为一个管道
	topicFunc func(v interface{}) bool //主题是一个过滤器
)

type Publisher struct {
	m           sync.RWMutex             //读写锁
	buffer      int                      // 订阅队列的缓存大小
	timeout     time.Duration            // 发布超时时间
	subscribers map[subscriber]topicFunc //订阅者信息
}

func NewPublisher(timeout time.Duration, buffer int) *Publisher {
	return &Publisher{
		buffer:      buffer,
		timeout:     timeout,
		subscribers: make(map[subscriber]topicFunc),
	}
}

// 增加一个新的订阅者, 订阅全部主题
func (p *Publisher) Subscribe() chan interface{} {
	return p.SubscribeTopic(nil)
}

// 增加新的订阅者, 订阅过滤后的主题
func (p *Publisher) SubscribeTopic(topic topicFunc) chan interface{} {
	ch := make(chan interface{}, p.buffer)
	p.m.Lock()
	p.subscribers[ch] = topic
	p.m.Unlock()

	return ch
}

func (p *Publisher) Evict(sub chan interface{}) {
	p.m.Lock()
	defer p.m.Unlock()

	delete(p.subscribers, sub)
	close(sub)
}

// 发布
func (p *Publisher) Publish(v interface{}) {
	p.m.RLock()
	defer p.m.RUnlock()

	var wg sync.WaitGroup
	for sub, topic := range p.subscribers {
		wg.Add(1)
		go p.sendTopic(sub, topic, v, &wg)
	}
	wg.Wait()
}

func (p *Publisher) Close() {
	p.m.Lock()
	defer p.m.Unlock()

	for sub := range p.subscribers {
		delete(p.subscribers, sub)
		close(sub)
	}
}
func (p *Publisher) sendTopic(sub subscriber, topic topicFunc, v interface{}, wg *sync.WaitGroup) {
	defer wg.Done()

	if topic != nil && !topic(v) {
		return
	}

	select {
	case sub <- v:
	case <-time.After(p.timeout):
	}
}

/*
控制并发数:
	go自带的godoc程序实现了一个vfs的包对应虚拟的的文件系统, 在vfs包下面有一个gatefs的子包, gatefs子包的
	目的就是为了控制访问虚拟文件系统最大的并发数.
	并发控制: 通过带缓存管道的发送和接收规则来实现最大并发阻塞

	vfs.OS(root string) FileSystem  //基于本地文件系统构造一个虚拟的文件系统
	gatefs.New(fs vfs.FileSystem, gateCh chan bool) vfs.FileSystem //基于现有的虚拟文件系统构造
	一个并发受控的虚拟文件系统.

	gatefs 对并发受控抽象了一个类型gate, 增加了enter和leave方法分别对应并发代码的进入和离开. 当超出并发数
	量超过限制的时候, enter方法会阻塞直到并发数量降下来为止.
*/


/*
并发的安全退出:
	Go语言中不同Goroutine之间主要依靠管道进行通信和同步.要同时处理多个管道的发送或接收操作,我们需要使用select关
	键字(这个关键字和网络编程中的select函数的行为类似). 当select有多个分支时,会随机选择一个可用的管道分支,如果
	没有可用的管道分支则选择default分支,否则会一直保存阻塞状态.

	// 管道超时判断
	select {
	case v := <-in:
		fmt.Println(v)
	case <-time.After(time.Second):
		return
	}

	// 非阻塞管道发生或接收
	select{
	case v := <-in:
		fmt.Println(v)
	default:

	}

	// 阻止main函数退出
	select{}
*/
