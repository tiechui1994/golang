package access


/*
config: 主要是基于ini的类型, 核心是map存储+常用的类型转换

effect: 决策

type Effector interface {
	// 将执行者收集的所有匹配结果合并为一个决策.
	MergeEffects(expr string, effects []Effect, results []float64) (bool, error)
}

注意: 默认的实现只是提供了4种决策模型, 需要进行扩展, 可以实现上述的Effector接口

model: 模型

persist: 持久化, 提供了适配器接口

rbac: 对于RBAC模型的特殊支持

Enforcer: 执行器, 整个casbin工作的核心结构体
*/