#include <stdio.h>
#include "extern_struct_clazz.h"

int print(char *str, Clazz post)
{
	printf("%s:(%d,%d,%d)\n", str, post.x, post.y, post.z);
	return 0;
}
