package access

/*
- config: 主要是基于ini的类型, 核心是map存储+常用的类型转换

- effect: 决策

type Effector interface {
	// 将执行者收集的所有匹配结果合并为一个决策.
	MergeEffects(expr string, effects []Effect, results []float64) (bool, error)
}

注意: 默认的实现只是提供了4种决策模型, 需要进行扩展, 可以实现上述的Effector接口



- model: 模型



- rbac: 对于RBAC模型的特殊支持
RoleManager, 提供了角色管理的上层接口
type RoleManager interface {
	// 清除所有存储的数据并将RoleManager重置为初始状态.
	Clear() error

	// 添加两个角色之间的继承链接. role:name1和role:name2. domain是角色的前缀(可用于其他目的)
	AddLink(name1 string, name2 string, domain ...string) error
	// 删除两个角色之间的继承链接
	DeleteLink(name1 string, name2 string, domain ...string) error
	// 判断两个角色之间是否有继承关系
	HasLink(name1 string, name2 string, domain ...string) (bool, error)

	// 获取用户继承的 "角色"
	GetRoles(name string, domain ...string) ([]string, error)
	// 获取继承角色的 "用户"
	GetUsers(name string, domain ...string) ([]string, error)
	// 打印所有的角色到日志
	PrintRoles() error
}

默认的RoleManager采用sync.Map进行存储.



- persist: 持久化, 提供了适配器接口

type Adapter interface {
	// 从存储当中加载所有的Policy Rule
	LoadPolicy(model model.Model) error
	// 将Policy Rule 同步到存储当中
	SavePolicy(model model.Model) error


	// 将策略规则添加到存储。
    // 这是自动保存功能的一部分
	AddPolicy(sec string, ptype string, rule []string) error
	// 从存储当中删除策略规则
	// 这是自动保存功能的一部分
	RemovePolicy(sec string, ptype string, rule []string) error
	// 从存储中删除与筛选器匹配的策略规则
	// 这是自动保存功能的一部分
	RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error
}

type Watcher interface {
	// 设置当其他实例更改了DB中的策略时,观察者将调用的回调函数.
	// 一个经典的回调是Enforcer.LoadPolicy()
	SetUpdateCallback(func(string)) error

	// 其他实例修改了策略, 调用此函数进行同步更新. 它通常在更改DB中的策略后调用, 如Enforcer.SavePolicy(),
	// Enforcer.AddPolicy(), Enforcer.RemovePolicy()等
	Update() error
}

默认的存储是文件存储.


- Enforcer: 执行器, 整个casbin工作的核心结构体
*/
