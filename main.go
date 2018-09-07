package main

import (
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/session"
	"fmt"
)
// Session 测试
func init() {
	managerConfig := session.ManagerConfig{
		CookieName: "gosessionid",
		EnableSetCookie: true,
		Gclifetime: 3600,
		Maxlifetime: 3600,
		Secure: false,
		CookieLifeTime: 3600,
		ProviderConfig: "",
	}
	globalSessions, _ := session.NewManager("memory", &managerConfig)
	fmt.Printf("%+v", globalSessions)
	go globalSessions.GC()
}
func main() {
	beego.BConfig.WebConfig.Session.SessionOn = true
	beego.Run()

}
