#include <stdio.h>
#include "extern_struct_clazz.h"
#include "extern_struct_print.h"

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