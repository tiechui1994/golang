# Model语法

- Model CONF至少应包含四个部分: `[request_definition]`, `[policy_definition]`, `[policy_effect]`, 
`[matchers]`.

- 如果 model 使用 RBAC, 还需要添加`[role_definition]`部分.

- Model CONF 可以包含注释. 注释以`#`开头, `#`将注释整行.


## request 定义

`[request_definition]` 部分用于request的定义, 它明确了 `e.Enforce()` 函数中参数的含义.

```
[request_definition]
r = sub, obj, act
```

`sub, obj, act`表示经典三元组: 访问实体(Subject), 访问资源(Object) 和 访问方法(Action).
但是, 可以自定义自己的请求表单, 如果不需要指定特定资源, 则可以这样定义 `sub, act`, 或者如果有两个
访问实体, 则为 `sub sub2, obj, act`


## policy定义

`[policy_definition]` 部分用于policy的定义, 以下文的mode配置为例:

```
[policy_definition]
p = sub, obj, act
p2 = sub, act
```

下面的内容是对policy规则的具体描述:

```
p, alice, data1, read
p2, bob, write-all-objects
```

policy部分的每一行称之为策略规则, 每条策略规则通常形如p, p2的`policy_type`开头. 如果存在多个
policy定义, 那么我们会根据前文提到的`policy_type`与具体的某条定义匹配. 上面的policy的绑定关系
将会在matcher中使用. 罗列如下:

```
(alice, data1, read) -> (p.sub, p.obj, p.act)
(bob, write-all-objects) -> (p2.sub, p2.act)
```

注1: 当前只支持形如 `p` 的单个policy定义, 形如 `p2` 类型的尚未支持. 通常状况下, 用户无需使用多个
policy定义.

注2: policy定义中的元素始终被视为字符串对待.


## policy effect定义

`[policy_effect]` 部分是对policy生效范围的定义, 原语定义了当多个policy rule同时匹配访问请求
request时, 该如何对多个决策结果进行集成以实现统一决策. 以下示例展示了**一个只有一条规则生效,其余都被拒绝
的情况:**

```
[policy_effect]
e = some(where (p.eft == allow))
```

该effect原语表示如果存在任意一个决策结果为`allow`的匹配规则, 则最终决策结果是 `allow`, 即allow-override.
其中`p.eft`表示策略的决策结果, 可以是`allow`或者`deny`, 当不指定规则的决策结果时, 取默认值 `allow`.
通常情况下, policy的`p.eft`默认为`allow`, 因此前面例子中都使用了这个默认值.


另外一个policy effect的例子:

```
[policy_effect]
e = !some(where (p.eft == deny))
```
该effect的原语表示不存在任何决策结果为 `deny` 的匹配规则, 则最终决策结果是 `allow`,  即deny-override.

**`some`** : 是否存在一条策略规则满足匹配器. 
**`any`** : 是否所有的策略规则都满足匹配器.
**`where`** : 条件, 后面的结果是一个bool值.


policy effect还可以利用逻辑运算符进行连接:

```
[policy_effect]
e = some(where (p.eft == allow)) && !some(where (p.eft == deny))
```

该effect原语表示当至少存在一个决策结果为 `allow` 的匹配规则, 且不存在决策结果为 `deny`的匹配规则时, 
则最终决策结果为 `allow`. 这时 `allow` 授权 和 `deny`授权同时存在, 但是 `deny` 优先.


## matchers

`[matchers]`原语定义了策略规则如何与访问请求进行匹配的匹配器, 其本质是布尔表达式, 可以理解为request,
policy 等原语定义了关于策略和请求的变量, 然后将这些变量代入matche原语中进行求值, 从而进行策略决策.

一个简单的例子:

```
[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act
```

该matchers原语表示, 访问请求request中的subject, object, action 三元组与策略规则policy rule
中的subject, object, action 三元组分别对应相同.

**`matchers`**原语支持 `+`, `-`, `*`, `/` 等数学运算, `==`, `!=`, `>`, `<`等关系运算符,
以及 `&&`(与), `||`(或), `!`(非)等逻辑运算符.

注: 虽然可以像其他原语一样的编写多个类似于 `m1`, `m2` 的 matcher, 但是当前只支持一个有效的matcher 
`m`. 通常情况下, 可以在一个matcher中使用上文提到的逻辑运算符来实现复杂的逻辑判断.


**matcher中的函数**

matcher的强大与灵活之处在于可以在matcher中定义函数, 这些函数可以是内置函数或自定义函数:

- 内置函数

| 函数 | 释义 | 示例 |
| --- | --- | --- |
| keyMatche(arg1, arg2) | 参数arg1是一个URL路径,例如`/alice/data1`,参数arg2可以是URL路径或者一个'*'模式,例如`/alice/*`,此函数返回arg1是否与arg2匹配 |  |
| keyMatche2(arg1, arg2) | 参数arg1是一个URL路径,例如`/alice/data1`,参数arg2可以是URL路径或者是一个':'模式,例如`/alice/:data`.此函数返回arg 是否与arg2匹配 |  |
| regexMatch(arg1, arg2) | 参数arg1可以是任何字符串,参数arg2是一个正则表达式.它返回arg1是否匹配arg2 |  |
| ipMatch(arg1, arg2) | 参数arg1是一个ip地址,如`192.168.1.100`,参数arg2可以是ip地址或者CIDR,如`192.168.1.0/24`.它返回arg1是否匹配arg2 |  |


- 自定义函数

首先, 准备好一个带有参数和返回值是布尔值的函数:

```
func keyMatch(arg1 string, arg2 string) bool {
    i := strings.Index(arg2, "*")
    if i == -1 {
        return arg1 == arg2
    }
    
    if len(arg1) > i {
        return args[:i] == arg2[:i]
    }
    
    return arg1 == arg2[:i]
}
```

然后, 使用interface{}类型包装此函数:

```
func KeyMatchFunc(args ...interface{}) (interface{}, error) {
    name1 := args[0].(string)
    name2 := args[1].(string)

    return (bool)(KeyMatch(name1, name2)), nil
}
```

最后, 将包装的函数注册到 casbin enforcer:

```
e.AddFunction("my_func", KeyMatchFunc)
```

现在, 在matcher当中使用:

```
[matchers]
m = r.sub == p.sub && my_func(r.obj, p.obj) && r.act == p.act
```


## role 定义

`[role_definition]`原语定义了RBAC中的角色继承关系.casbin支持RBAC系统的多个实例, 例如, 用户可以有角色和继承关系,
而资源也可以有角色和继承关系. 这两个RBAC系统不会干扰.

```
[role_definition]
g = _, _
g2 = _, _
```

上述Role原语表示 `g` 是一个RBAC体系, `g2` 是另一个RBAC体系. `_, _` 表示角色继承关系的前项和后项, 即前项继承后项角色
的权限. 一般来讲, 如果你需要进行角色和用户的绑定, 直接使用 `g` 即可. 当需要表示角色(或者组)与用户和资源的绑定关系时, 可以
使用 `g` 和 `g2`这样的表现形式. 

在casbin里, 我们以policy表示中实际的用户-角色映射关系(或者资源-角色映射关系), 案例:

```
p, admin, data_admin, read
g, alice, data_admin
```

上述策略规则表示`alice`继承或具有角色 `data_admin`, 这里的alice可以为具体的某个用户, 某种资源亦或是某个角色, 在casbin
中它将被当作字符串来对待.

在matchers当作, 应该以如下方式来校验角色信息:

```
[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
```

它表示请求request中 `sub`必须具有policy中定义的 `sub` 角色.

注:
```
1.casbin只存储用户角色的映射关系.
2.casbin不验证用户是否为有效用户, 或者角色是否为有效角色. 这应该由身份验证来处理.
3.RBAC系统中的用户名称和角色名称不应相同. 因为casbin将用户名和角色识别为字符串, 所以当前语境下
casbin无法得出这个字面量到底指代用户 alice 还是角色 alice. 这时, 使用明确的 role_alice,
问题便可迎刃而解.
4.假设A具有角色B, B具有角色C,并且A有角色C, 这种传递性在当前版本会造成死循环.
```


## 域租户的角色定义

在casbin中的RBAC角色可以是全局域或者基于特定域的. 特定域的角色意味着当用户处于不同的域/租户群体时,
用户所表现的角色也不尽相同. 这对于像云服务这样的大型系统非常有用, 因为用户通常分属于不同的租户群体.

域/租户的角色定义:

```
[role_definition]
g = _, _, _
```

原语中的第三个 `_`表示域的概念, 相对应的策略规则的实例如下:

```
p, admin, tenant1, data1, read
p, admin, tenant2, data2, read

g, alice, admin, tenant1
g, alice, user, tenant2
```

该实例表示tenant1的域内角色admin可以读取data1, alice在tenant1域中具有admin角色, 但在tenant2域中具有user角色, 
所以alice可以有读取data1的权限. 同理, 因为alice不是tenant2的admin,所以她访问不了data2.


接下来在matcher中,应该像下面的例子一样检查角色信息:

```
[matchers]
m = g(r.sub, p.sub, r.dom) && r.dom == p.dom && r.obj == p.obj && r.act == p.act
```