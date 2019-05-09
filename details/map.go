package details

/*
map 的 key 类型: map的key的类型只要支持 `==` 和 `!=` 操作即可, 即Comparabale
	- bool
	- number
	- string
	- pointer
	- channel
	- interface
	- struct
	- array (Type 是上述类型)

map的 key 不能是以下类型:
	- slice
	- map
	- function
**/