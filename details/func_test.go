package details

import (
	"testing"
	"fmt"
)

func TestDeferExecute(t *testing.T) {
	DeferExecute()
}

func TestDeferInit(t *testing.T) {
	DeferInit()
	fmt.Println("======================")
	DeferParam()
}

func TestMapPointer(t *testing.T) {
	ForRange()
}

func TestSlice(t *testing.T) {
	Slice()
}

func TestConcurrentMap(t *testing.T) {
	ConcurrentMap()
}

func TestUseType(t *testing.T) {
	UseType()
}

func TestDeferAndScope(t *testing.T) {
	DeferAndScope()
}

func TestEqualStruct(t *testing.T) {
	EqualStruct()
}

func TestScope(t *testing.T) {
	Scope()
}
