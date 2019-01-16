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
