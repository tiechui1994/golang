package access

/*
- config: 主要是基于ini的类型, 核心是map存储+常用的类型转换

核心存储结构: map[string]map[string]string, 即 section: key=value

其中:
	section: request_definition, policy_definition, role_definition, policy_effect, matchers
	key: r, p, g(g2,g3,...), e, m
	value: ...



- effect: 决策

type Effector interface {
	// 将执行者收集的所有匹配结果合并为一个决策.
	MergeEffects(expr string, effects []Effect, results []float64) (bool, error)
}

注意: 默认的实现只是提供了4种决策模型, 需要进行扩展, 可以实现上述的Effector接口



- model: 模型

// Model: 整个访问控制模型.
type Model map[string]AssertionMap

// AssertionMap是断言的集合, 可以是"r","p","g","e","m".
type AssertionMap map[string]*Assertion

// Assertion: 模型的一部分中的表达式.
// For example: r = sub, obj, act
type Assertion struct {
	Key    string
	Value  string
	Tokens []string
	Policy [][]string
	RM     rbac.RoleManager
}


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


	// 将策略规则添加到存储.
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

type Enforcer struct {
	modelPath string             // model的配置文件路径
	model     model.Model	     // model配置文件加载的后形成的Model
	fm        model.FunctionMap  // model当中使用的函数
	eft       effect.Effector    // 决策器, 默认是 effect.DefaultEffector

	adapter persist.Adapter // 适配器, 主要是针对Policy变更或者存储的变化, casbin自身实现了基于文件的Adaptec
	watcher persist.Watcher // 监测
	rm      rbac.RoleManager // 针对RBAC模型

	enabled            bool	// casbin状态(是否可用), 当不可用时所有的请求决策都是允许. true
	autoSave           bool // 针对Policy改变是否自动保存. true
	autoBuildRoleLinks bool // 自动构建角色的继承关系. true
}

// 构建决策实例
NewEnforcer(params ...interface{})
参数说明:
	enableLog: 是否启动log

	modelPath: 配置路径
	enableLog: 是否启动log

	model: Model
	enableLog: 是否启动log

	modelPath: 配置路径
	policyPath: 策略文件路径
	enableLog: 是否启动log

	modelPath: 配置路径
	adapter: Adapter
	enableLog: 是否启动log

	model: Model
	adapter: Adapter
	enableLog: 是否启动log

// 决策
Enforce(rvals ...interface{})
1)使用govaluate构建表达式
2)使用policy解析matcher, 得到一个个决策结果
3)汇总决策结果, 返回
*/
