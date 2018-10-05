package logging

import (
	"math/rand"
	"testing"
)

func TestLogging(t *testing.T) {
	log := GetBeeLogger()
	var config = `{
					"filename":"beego.log",
					"maxLines":200,
					"maxsize":3712594255,
					"maxFiles":20
                  }`

	log.SetLogger(AdapterFile, config)
	for i := 0; i < 1000; i++ {
		log.Debug("%d", rand.Int())
	}
}

func TestSmTP(t *testing.T) {
	log := GetBeeLogger()
	var config = `{
					"username":"beego.log",
					"password":"",
					"host":"",
					"sendTos":[""]
                  }`

	log.SetLogger(AdapterMail, config)
	for i := 0; i < 1000; i++ {
		log.Debug("%d", rand.Int())
	}
}
