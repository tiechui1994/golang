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
extern Clazz fun(Clazz x, Clazz y); // 接口函数声明
#endif