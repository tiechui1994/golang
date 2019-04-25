package main

import (
	"reflect"
	"fmt"
)

// 反射获取设置Field值
// 注意: object 必须是指针, 否则后续的值是无法设置上的; value的值的类型需要与被设置的字段的类型一致
func SetFieldValue(object interface{}, field string, value interface{}) {
	val := reflect.ValueOf(object)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() == reflect.Struct {
		fieldVal := val.FieldByName(field)
		if fieldVal.CanSet() && fieldVal.Type().ConvertibleTo(reflect.TypeOf(value)) {
			fieldVal.Set(reflect.ValueOf(value))
		}
	}
}

// 反射执行方法
// 注意: object必须是指针类型, params的参数列表和真实的参数列表必须一致
func Execute(object interface{}, method string, params ...interface{}) (ok bool, returns []interface{}) {
	val := reflect.ValueOf(object)

	if val.Kind() == reflect.Ptr {
		methodVal := val.MethodByName(method)
		if methodVal.Kind() == reflect.Func && len(params) == methodVal.Type().NumIn() {
			methodTyp := methodVal.Type()
			in := make([]reflect.Value, len(params))
			for i := range params {
				paramVal := reflect.ValueOf(params[i])
				if paramVal.Type().ConvertibleTo(methodTyp.In(i)) {
					in[i] = paramVal
					continue
				}

				return false, nil
			}

			out := methodVal.Call(in)
			returns = make([]interface{}, len(out))
			for i := range out {
				returns[i] = out[i].Interface()
			}

			return true, returns
		}
	}

	return false, nil
}

type X struct {
	A string
	B int
	C bool
	D []byte
	F chan string
	H struct {
		P int
		V uint
	}
}

func (x *X) Print(name string) {
	fmt.Println("Hello World, ", name)
}

func main() {
	x := &X{}
	a := "Hello"
	SetFieldValue(x, "A", a)
	fmt.Printf("After Set A:%+v \n", x.A)

	SetFieldValue(x, "D", []byte("Hello"))
	fmt.Printf("After Set D:%+v \n", x.D)

	f := make(chan string)
	SetFieldValue(x, "F", f)
	fmt.Printf("After Set F:%+v \n", x.F)

	h := struct {
		P int
		V uint
	}{
		P: 100,
		V: 200,
	}
	SetFieldValue(x, "H", h)
	fmt.Printf("After Set H:%+v \n", x.H)

	Execute(x, "Print", "JAVA")
}
