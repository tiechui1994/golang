package env

import (
	"fmt"
	"os"
	"strings"
	"golang/web/utils"
)

/**
 加载系统环境变量作为配置文件.
*/

var env *utils.BeeMap

// 加载环境变量
func init() {
	env = utils.NewBeeMap()
	for _, e := range os.Environ() {
		splits := strings.Split(e, "=")
		env.Set(splits[0], os.Getenv(splits[0]))
	}
}

func Get(key string, defVal string) string {
	if val := env.Get(key); val != nil {
		return val.(string)
	}
	return defVal
}

func MustGet(key string) (string, error) {
	if val := env.Get(key); val != nil {
		return val.(string), nil
	}
	return "", fmt.Errorf("no env variable with %s", key)
}

// env是环境变量在内存当中的一个拷贝
// Set只会影响到当前进程使用的环境变量的值, 对于其他进程没有影响
func Set(key string, value string) {
	env.Set(key, value)
}

// 在Set的基础上, 会修改所有进程中环境变量的值
func MustSet(key string, value string) error {
	err := os.Setenv(key, value)
	if err != nil {
		return err
	}
	env.Set(key, value)
	return nil
}

// 获取当前进程中是所有环境变量(内存当中经过修改的那份)
func GetAll() map[string]string {
	items := env.Items()
	envs := make(map[string]string, env.Count())

	for key, val := range items {
		switch key := key.(type) {
		case string:
			switch val := val.(type) {
			case string:
				envs[key] = val
			}
		}
	}
	return envs
}
