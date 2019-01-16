package main

//#include <stdio.h>
import "C"

//export helloString
func helloString(s string) {}

//export helloSlice
func helloSlice(s []byte) {}
