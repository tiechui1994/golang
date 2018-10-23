package memory

/**
内存块:
	分配器将其关联的内存块分为两种:
	span: 由多个地址连续的页(page)组成的大块内存.
	object: 讲span按特定大小切分成多个小块, 每个小块可以存储一个对象.

	分配器按页数来区分大小不同的span. 以页数为单位将span存放到关联数组中, 需要时就以页数为索引
	进行查找. 当然span大小并非固定不变, 在获取闲置span时, 如果没有找到大小合适的, 那么就返回页
	数更多的, 此时会引发裁剪操作, 多余部分讲构成新的span被放回管理数组. 分配器还会尝试讲相邻的空
	闲span合并, 以构成更大的内存块, 减少碎片. (bitmap)

	分配器的数据结构是:
	fixalloc: 固定大小的堆外对象的空闲列表分配器, 用于管理分配器使用的存储.

	mheap: malloc堆, 管理闲置的span,需要时向操作系统申请内存.
	mcache: 每个运行期工作线程都会绑定一个cache, 用于无锁object分配
	mcentral: 为所有的cache提供切分好的后备span资源

	mspan: 由mheap管理的一系列页面.

	mstats: 分配统计信息.

	分配小对象:
	mcache -> mcentral -> mheap -> os

	1.计算分配对象对应的size, 并查找当前P的mcache中相应的mspan. 扫描mspan的free bitmap 查找空闲slot.
	如果有空闲slot, 分配它.这可以在不获取锁定的情况下完成.

	2.如果mspan没有空闲slot, 从mcentral的mspans列表中获取一个给定大小的空闲的mspan。
	获得整个跨度可以分摊锁定中心的成本.

	3.如果mcentral的mspan列表为空,则从mheap获取一系列页面以用于mspan。

	4.如果mheap为空或没有足够大的页面运行, 从操作系统分配一组新页面(至少1MB). 分配大量页面会分摊与
	操作系统通信的成本.

	释放对象:
	1.如果mspan正在响应分配而被扫描，则返回到mcache以满足分配。

*/
