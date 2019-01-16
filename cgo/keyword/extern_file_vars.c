#include <stdio.h>

int max(int x, int y); // 函数提前声明
void say();

int main(int argc, char *argv[])
{
	int result;
	extern int X; // 扩展X的作用域,
	extern int Y;
	result = max(X, Y);
	X=100;
	Y=200;
	printf("the max value is %d\n", result);

	say();
	return 0;
}

int X = 10; // 定义外部变量
int Y = 20;

int max(int x, int y)
{
	return (x > y ? x : y);
}

void say() {
    printf("the X: %d, Y:%d\n", X, Y);
}