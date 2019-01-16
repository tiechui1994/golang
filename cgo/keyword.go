package cgo

/*****************************************
编译:
	C++ 编译: g++ xxx.cpp -o xxx
	C 编译:   gcc xxx.c -o  xxx
******************************************/

/***********************************************************************************************************************

C当中的关键字:

1. static
static是C中常用的修饰符, 它被用来控制变量的存贮方式和可见性.

所谓的static对象, 其寿命是 `从构造出来到程序结束为止`. 因此stack和heap-base对象都被排除,
这种对象包括global对象, 定义于namespace作用域内的对象, 在classes内, 在函数内, 以及在file作用域内被声明为static的对象.

1) 存在于全局作用域中的静态变量

```
// 全局可访问, 在内存中只有一份拷贝, 可被很多函数修改.
#include <iostream>

static int i = 1; // 作用域是整个file

void get(){
    std::cout << "in func , the i is " << i << std::endl;
}

int main(){
        std::cout << "the i is " << i << std::endl;
        get();
        return 0;
}
```

2) 存在于函数当中的静态变量

```
// 只能在这个函数中才能被调用.(类似于闭包当中的变量)
// 函数调用结束, 一般局部变量都被回收了, 静态变量还存在.
#include <iostream>

void get(){
        static int i = 1;
        std::cout << "the i is " << i << std::endl;
        i++;
}

int main(){
        get(); // i = 1
        get(); // i = 2
        std::cout << "the i is " << i << std::endl; // todo: 这种是错误的
        return 0;
}
```

3) 存在于类的成员变量中的静态变量

```
// 其实原理和函数中的静态变量类似, 类实例化出来的对象被销毁后, 类变量(静态成员变量)依旧存在于内存当中.
#include <iostream>

class Widget{
public:
    Widget(int i){
	    a = i;
    }
    void get();
private:
    static int a;  // 声明静态变量
};

int Widget::a = 1;  // 由于是类变量不是属于专属于一个对象的, 被所有对象共享, 所以需要在类外定义.
void Widget::get(){
    std::cout << "current a:" << a++ << std::endl;
}

int main(){
	Widget w(10);
	w.get(); // a = 1
	w.get(); // a = 2
	return 0;
}
```

4) 存在于类中成员函数中的静态变量

```
#include <iostream>

class widget{
 public:
    widget(){}
    void get();
};

void widget::get(){
    static int i = 1;
    // 成员函数中的静态变量的作用域范围跟普通局部变量的作用域范围是一样的
    std::cout << "in func, the i is " << i++ << std::endl;
}

int main(int argc, char const* argv[])
{
    widget w1;
    w1.get();  // in func, the i is 1
    widget w2;
    w2.get(); // in func, the i is 2
    return 0;
}
```

5) 存在于命令空间中的静态变量

```
#include <iostream>

// todo: namespace Widget
namespace Widget {
    static int i = 1;  // 在该命名空间可用

    void get(){
    	std::cout << "the i is " << i++ << std::endl;
    }
}

int main (){
    using namespace Widget;
    get();  //the i is 1
    get(); // the i is 2
    return 0;
}
```

6) 存在于全局作用域的静态函数

```
// todo: 和一般的函数差不多, 但是它将该函数的链接属性限制为内链接, 只能在本编译单元中使用(也就是本文件), 不能被extern等在外部文件中引用.
static void get(){
    std::cout << "this is staic global func" << std::endl;
}

int main(){
    get();
    get();
    return 0;
}
```

7) 存在于类中的静态函数

```
#include <iostream>

class Widget{
    public:
    	Widget(int i){
        	a = i;
   	 	}
    	static void get(); // 声明静态函数

    private:
    	static int a;
    	int b;
};

int Widget::a = 1;
void Widget::get(){
    std::cout << b << std::endl;  // todo: 错误, 在类中的静态函数只能调用类的静态方法以及使用类的静态变量. `非静态成员没有实例化`.
    std::cout << a << std::endl; // ok
}

int main(){

    Widget w(1);
    w.get();
    return 0;
}
```

补充: 链接, 分为三种, 内链接, 外链接 和 无链接

内链接:

```
static 修饰的函数和变量 以及 const 修饰的变量(不包含extern) 都是内链接, 只能在本文件中使用, 即使别的
文件定义了相同的变量名也不要紧.
```

外链接:

```
没有用static修饰的全局变量或者函数, 都可以作为外链接.
用extern修饰的全局变量或者函数, 也可以作为外链接.
特殊变量, extern const int i = 1, 也是外部链接. (extern的作用会覆盖掉const使它成为外链接)
```

无链接:

```
局部变量, 其生命周期只是在函数执行期间, 因此没有链接属性.
```

2. extern
extern, "C"是使C++能够调用C写的库文件的一个手段,如果要对编译器提示使用C的方式来处理函数的话, 那么就要使用extern "C"来说明.

1) 用 extern 声明外部变量

注: 这里的外部变量是指在函数或者文件外部定义的'全局变量'. 外部变量定义必须在所有的函数之外, 且只能定义一次.

1.1) 在一个文件内声明的外部变量(扩展变量的作用域)

注: C当中变量或者函数是先声明, 后使用.

作用域: 如果在变量定义之前要使用该变量, 则在用之前加 extern 声明变量, 作用域扩展到从声明开始, 到本文件结束.

案例:
```c
#include <stdio.h>

int max(int x, int y); // 函数提前声明
int main(int argc, char *argv[])
{
	int result;
	extern int X; // 扩展X的作用域,
	extern int Y;
	result = max(X, Y);
	printf("the max value is %d\n", result);
	return 0;
}

int X = 10; // 定义外部变量
int Y = 20;

int max(int x, int y)
{
	return (x > y ? x : y);
}
```

注:  这里的extern类似于Python当中的global. 但是却不同, C当中的extern是修改了变量的作用域范围, 使得全局变量
在局部可被访问和修改. Python当中的global强调的是在局部修改全局变量, 而非访问(天然可以访问).

1.2) 在多个文件中声明外部变量

作用域: 如果整个工程有多个文件组成, 在一个文件中引用另外一个文件中已经定义的外部变量时, 则只需在引用变量的文件中使
用extern关键字加以声明即可. 可见, 其作用域从一个文件扩展到多个文件了.

案例:
a.c, 定义变量
```
int BASE = 100;
```

b.c，使用变量
```
#include <stdio.h>
extern int BASE; // todo: 声明的外部变量

int main(int argc, char *argv[])
{
	printf("BASE: %d\n", BASE);
	return 0;
}
```
注: 引用外部变量和通过函数形参值传递变量的区别: 用extern 引用外部变量, 可以在引用的模块内修改其值; 而形参值
传递的变量则不能修改其值, 除非是地址传递.

因此,如果多个文件同时对需要应用的变量进行同时操作, 可能会修改该变量, 类似于形参的地址传递, 从而影响其他模块的
使用,因此要慎重的使用.

1.3) 在多个文件中声明外部结构体变量(声明外部变量, 类似简单变量声明)

案例:

clazz.h, 结构体的声明, 函数声明, 外部结构体变量声明.
clazz.c, 实现了函数声明
```
// clazz.h
#ifndef __B_H
#define __B_H

#if 1
typedef struct {
	int x;
	int y;
	int z;
} Clazz;
#endif

extern Clazz localPost;  // todo: 外部结构体变量声明
extern Clazz fun(Clazz x, Clazz y); // todo: 接口函数声明
#endif

// clazz.c
#include <stdio.h>
#include "clazz.h"

Clazz fun(Clazz first, Clazz next)
{
	Clazz ret;
	ret.x = next.x - first.x;
	ret.y = next.y - first.y;
	ret.z = next.z - first.z;
	return ret;
}
```

print.h: 函数声明, 依赖clazz.h
print.c: 实现了函数
```
// print.h
#ifndef __C_H
#define __C_H
#include "clazz.h"
extern int print(char *, Clazz post);
#endif

// print.c
#include <stdio.h>
#include "clazz.h"

int print(char *str, Clazz post)
{
	printf("%s:(%d,%d,%d)\n", str, post.x, post.y, post.z);
	return 0;
}
```

main.c: 初始化结构体变量, 调用声明的函数
```
// main.c
#include <stdio.h>
#include "clazz.h"
#include "print.h"

Clazz localPost = {1, 2, 9};
Clazz nextPost  = {3, 8, 6};

int main(int argc, char *argv[])
{
	Clazz ret;
	print("fist point", localPost);
	print("second point", nextPost);
	ret = fun(localPost, nextPost);
	printf("the vector is (%d %d %d)\n", ret.x, ret.y, ret.z);
	return 0;
}
```

2) 使用extern声明外部函数

定义函数时, 在函数的返回值类型前面加上extern关键字, 表示此函数是外部函数, 可供其他文件调用,
如, extern int func(int x, int y), C语言规定, 此时extern可以省略, 隐形为外部函数.

调用函数时, 需要用extern对函数做出声明. C语言中规定,声明时可以省略extern.

作用域: 使用extern声明能够在一个文件中调用其他文件的函数, 即把被调用的函数的作用域扩展到本文件.

***********************************************************************************************************************/
