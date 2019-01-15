package cgo

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

***********************************************************************************************************************/
