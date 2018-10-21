package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
)

/**
 json方式的配置文件, 文件的解析和保存比较简单.
 Set()只支持单一深度的设置
 Get()支持多深度的获取
*/

// 实现了Config接口
type JSONConfig struct {
}

// 解析json文件
func (js *JSONConfig) Parse(filename string) (Configer, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return js.ParseData(content)
}

// 核心:
func (js *JSONConfig) ParseData(data []byte) (Configer, error) {
	x := &JSONConfigContainer{
		data: make(map[string]interface{}),
	}
	err := json.Unmarshal(data, &x.data) // json对象
	if err != nil { // 数组
		var wrappingArray []interface{}
		err2 := json.Unmarshal(data, &wrappingArray)
		if err2 != nil {
			return nil, err
		}
		x.data["rootArray"] = wrappingArray
	}

	x.data = ExpandValueEnvForMap(x.data)

	return x, nil
}

//-------------------------------------------------------------------------------------------

// 存储JSON的数据
type JSONConfigContainer struct {
	data map[string]interface{}
	sync.RWMutex
}

// 辅助函数, 获取Raw数据
func (c *JSONConfigContainer) getData(key string) interface{} {
	if len(key) == 0 {
		return nil
	}

	c.RLock()
	defer c.RUnlock()

	// 支持格式:section1::section2::key的查找
	sectionKeys := strings.Split(key, "::")
	if len(sectionKeys) >= 2 {
		curValue, ok := c.data[sectionKeys[0]]
		if !ok {
			return nil
		}

		// 支持深度查找
		for _, key := range sectionKeys[1:] {
			if v, ok := curValue.(map[string]interface{}); ok {
				if curValue, ok = v[key]; !ok {
					return nil
				}
			}
		}
		return curValue
	}

	// 只是单纯的key
	if v, ok := c.data[key]; ok {
		return v
	}
	return nil
}

func (c *JSONConfigContainer) Bool(key string) (bool, error) {
	val := c.getData(key)
	if val != nil {
		return ParseBool(val)
	}
	return false, fmt.Errorf("not exist key: %q", key)
}

func (c *JSONConfigContainer) DefaultBool(key string, defaultval bool) bool {
	if v, err := c.Bool(key); err == nil {
		return v
	}
	return defaultval
}

func (c *JSONConfigContainer) Int(key string) (int, error) {
	val := c.getData(key)
	if val != nil {
		if v, ok := val.(float64); ok {
			return int(v), nil
		}
		return 0, errors.New("not int value")
	}
	return 0, errors.New("not exist key:" + key)
}

func (c *JSONConfigContainer) DefaultInt(key string, defaultval int) int {
	if v, err := c.Int(key); err == nil {
		return v
	}
	return defaultval
}

func (c *JSONConfigContainer) Int64(key string) (int64, error) {
	val := c.getData(key)
	if val != nil {
		if v, ok := val.(float64); ok {
			return int64(v), nil
		}
		return 0, errors.New("not int64 value")
	}
	return 0, errors.New("not exist key:" + key)
}

func (c *JSONConfigContainer) DefaultInt64(key string, defaultval int64) int64 {
	if v, err := c.Int64(key); err == nil {
		return v
	}
	return defaultval
}

func (c *JSONConfigContainer) Float(key string) (float64, error) {
	val := c.getData(key)
	if val != nil {
		if v, ok := val.(float64); ok {
			return v, nil
		}
		return 0.0, errors.New("not float64 value")
	}
	return 0.0, errors.New("not exist key:" + key)
}

func (c *JSONConfigContainer) DefaultFloat(key string, defaultval float64) float64 {
	if v, err := c.Float(key); err == nil {
		return v
	}
	return defaultval
}

func (c *JSONConfigContainer) String(key string) string {
	val := c.getData(key)
	if val != nil {
		if v, ok := val.(string); ok {
			return v
		}
	}
	return ""
}

func (c *JSONConfigContainer) DefaultString(key string, defaultval string) string {
	// TODO FIXME should not use "" to replace non existence
	if v := c.String(key); v != "" {
		return v
	}
	return defaultval
}

// 使用 ";" 分割形成的字符串数组
func (c *JSONConfigContainer) Strings(key string) []string {
	stringVal := c.String(key)
	if stringVal == "" {
		return nil
	}
	return strings.Split(c.String(key), ";")
}

func (c *JSONConfigContainer) DefaultStrings(key string, defaultval []string) []string {
	if v := c.Strings(key); v != nil {
		return v
	}
	return defaultval
}

// 第一层key
func (c *JSONConfigContainer) GetSection(section string) (map[string]string, error) {
	if v, ok := c.data[section]; ok {
		return v.(map[string]string), nil
	}
	return nil, errors.New("nonexist section " + section)
}

func (c *JSONConfigContainer) SaveConfigFile(filename string) (err error) {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.MarshalIndent(c.data, "", "  ")
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	return err
}

// 只支持第一层次的存储
func (c *JSONConfigContainer) Set(key, val string) error {
	c.Lock()
	defer c.Unlock()
	c.data[key] = val
	return nil
}

func (c *JSONConfigContainer) DIY(key string) (v interface{}, err error) {
	val := c.getData(key)
	if val != nil {
		return val, nil
	}
	return nil, errors.New("not exist key")
}

func init() {
	Register("json", &JSONConfig{})
}
