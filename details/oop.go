package details

// golang 面向对象的继承机制

/*
类与类型

golang中没有class关键字, 却引入了type, 二者不是简单的替换那么简单, type表达的涵义远比class要广. 主流
的面向对象语言(C++, Java)不太强调类与类型的区别, 本着一切皆对象的原则, 类被设计成了一个对象生成器. 这些语
言中的 type 是以 class 为基础的, 即通过 class 来定义 type, class 是这类语言的根基. 与之不同, golang
中更强调 type, 在golang中根本看不到 class 的影子. 在实现上, 传统语言(C++, Java) 的 class 只是 type
功能的一部分:

*/
