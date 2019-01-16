#include <stdio.h>
#include "extern_struct_clazz.h"

Clazz fun(Clazz first, Clazz next)
{
	Clazz ret;
	ret.x = next.x - first.x;
	ret.y = next.y - first.y;
	ret.z = next.z - first.z;
	return ret;
}