// Usage:
//  import "github.com/astaxie/beego/config"
//Examples.
//
//  cnf, err := config.NewConfig("ini", "config.conf")
package config

import (
	"fmt"
	"os"
	"reflect"
	"time"
)

// Config 顶层设计, Map结构
type Configer interface {
	Set(key, val string) error // 支持section::key为主键的插入(ini文件)

	// 支持section::key为主键(ini文件)
	String(key string) string
	Strings(key string) []string
	Int(key string) (int, error)
	Int64(key string) (int64, error)
	Bool(key string) (bool, error)
	Float(key string) (float64, error)

	// 支持section::key为主键(ini文件)
	DefaultString(key string, defaultVal string) string
	DefaultStrings(key string, defaultVal []string) []string
	DefaultInt(key string, defaultVal int) int
	DefaultInt64(key string, defaultVal int64) int64
	DefaultBool(key string, defaultVal bool) bool
	DefaultFloat(key string, defaultVal float64) float64
	DIY(key string) (interface{}, error)
	GetSection(section string) (map[string]string, error) // 获取 Sesction

	SaveConfigFile(filename string) error
}

// Config 引擎, 配置文件 -> 配置Config
type Config interface {
	Parse(key string) (Configer, error)
	ParseData(data []byte) (Configer, error)
}

var adapters = make(map[string]Config)

func Register(name string, adapter Config) {
	if adapter == nil {
		panic("config: Register adapter is nil")
	}
	if _, ok := adapters[name]; ok {
		panic("config: Register called twice for adapter " + name)
	}
	adapters[name] = adapter
}

// -------------------------------------------------------------------------------------------------------------

// adapterName 可以是 ini/json/xml/yaml, 对于xml/yaml需要安装依赖文件
// 从文件构建Config
func NewConfig(adapterName, filename string) (Configer, error) {
	adapter, ok := adapters[adapterName]
	if !ok {
		return nil, fmt.Errorf("config: unknown adaptername %q (forgotten import?)", adapterName)
	}
	return adapter.Parse(filename)
}

// 从配置数据data构建Config
func NewConfigData(adapterName string, data []byte) (Configer, error) {
	adapter, ok := adapters[adapterName]
	if !ok {
		return nil, fmt.Errorf("config: unknown adaptername %q (forgotten import?)", adapterName)
	}
	return adapter.ParseData(data)
}

// ExpandValueEnvForMap convert all string value with environment variable.
// 将
func ExpandValueEnvForMap(m map[string]interface{}) map[string]interface{} {
	for k, v := range m {
		switch value := v.(type) {
		case string:
			m[k] = ExpandValueEnv(value)
		case map[string]interface{}:
			m[k] = ExpandValueEnvForMap(value)
		case map[string]string:
			for k2, v2 := range value {
				value[k2] = ExpandValueEnv(v2)
			}
			m[k] = value
		}
	}
	return m
}

// ExpandValueEnv returns value of convert with environment variable.
//
// Return environment variable if value start with "${" and end with "}".
// Return default value if environment variable is empty or not exist.
//
// It accept value formats "${env}" , "${env||}}" , "${env||defaultValue}" , "defaultvalue".
// Examples:
//	v1 := config.ExpandValueEnv("${GOPATH}")			// return the GOPATH environment variable.
//	v2 := config.ExpandValueEnv("${GOAsta||/usr/local/go}")	// return the default value "/usr/local/go/".
//	v3 := config.ExpandValueEnv("Astaxie")				// return the value "Astaxie".
func ExpandValueEnv(value string) (realValue string) {
	realValue = value

	vLen := len(value)
	// 3 = ${}
	if vLen < 3 {
		return
	}
	// Need start with "${" and end with "}", then return.
	if value[0] != '$' || value[1] != '{' || value[vLen-1] != '}' {
		return
	}

	key := ""
	defaultV := ""
	// value start with "${"
	for i := 2; i < vLen; i++ {
		if value[i] == '|' && (i+1 < vLen && value[i+1] == '|') {
			key = value[2:i]
			defaultV = value[i+2: vLen-1] // other string is default value.
			break
		} else if value[i] == '}' {
			key = value[2:i]
			break
		}
	}

	realValue = os.Getenv(key)
	if realValue == "" {
		realValue = defaultV
	}

	return
}

// ParseBool returns the boolean value represented by the string.
//
// It accepts 1, 1.0, t, T, TRUE, true, True, YES, yes, Yes,Y, y, ON, on, On,
// 0, 0.0, f, F, FALSE, false, False, NO, no, No, N,n, OFF, off, Off.
// Any other value returns an error.
func ParseBool(val interface{}) (value bool, err error) {
	if val != nil {
		switch v := val.(type) {
		case bool:
			return v, nil
		case string:
			switch v {
			case "1", "t", "T", "true", "TRUE", "True", "YES", "yes", "Yes", "Y", "y", "ON", "on", "On":
				return true, nil
			case "0", "f", "F", "false", "FALSE", "False", "NO", "no", "No", "N", "n", "OFF", "off", "Off":
				return false, nil
			}
		case int8, int32, int64:
			strV := fmt.Sprintf("%d", v)
			if strV == "1" {
				return true, nil
			} else if strV == "0" {
				return false, nil
			}
		case float64:
			if v == 1.0 {
				return true, nil
			} else if v == 0.0 {
				return false, nil
			}
		}
		return false, fmt.Errorf("parsing %q: invalid syntax", val)
	}
	return false, fmt.Errorf("parsing <nil>: invalid syntax")
}

// ToString converts values of any type to string.
func ToString(x interface{}) string {
	switch y := x.(type) {

	// Handle dates with special logic
	// This needs to come above the fmt.Stringer
	// test since time.Time's have a .String()
	// method
	case time.Time:
		return y.Format("A Monday")

		// Handle type string
	case string:
		return y

		// Handle type with .String() method
	case fmt.Stringer:
		return y.String()

		// Handle type with .Error() method
	case error:
		return y.Error()

	}

	// Handle named string type
	if v := reflect.ValueOf(x); v.Kind() == reflect.String {
		return v.String()
	}

	// Fallback to fmt package for anything else like numeric types
	return fmt.Sprint(x)
}
