package util

import (
	"testing"
	"fmt"
)

func TestGetGID(t *testing.T) {
	fmt.Println(GetGID())
}

func TestIdentity(t *testing.T) {
	ok, addr, birth, sex := Identity("612527198911120652")
	t.Log("isok", ok)
	t.Log("addr", addr)
	t.Log("birth", birth)
	t.Log("sex", sex)
}
