package config

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

/**
IniConfig 和 IniConfigContainer 是孪生兄弟,
IniConfig 负责解析ini文件
IniConfigContainer 负责存储解析后的结果
*/

var (
	defaultSection = "default" // 默认的section, 即ini文件当中的项没有指定section, 则归结到[default]当中

	bNumComment = []byte{'#'} // 注释开始符号
	bSemComment = []byte{';'} // 注释开始符号
	bEmpty      = []byte{}
	bEqual      = []byte{'='}
	bDQuote     = []byte{'"'}

	sectionStart = []byte{'['} // section 开始标志
	sectionEnd   = []byte{']'} // section 结束标志

	lineBreak = "\n"
)

// 实现了Config接口, 专门解析ini文件
type IniConfig struct {
}

// 返回的是 Configer, 即ini文件对应在内存当中的存储结构, IniConfigContainer
func (ini *IniConfig) Parse(filePath string) (Configer, error) {
	return ini.parseFile(filePath)
}

func (ini *IniConfig) parseFile(filePath string) (*IniConfigContainer, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return ini.parseData(filepath.Dir(filePath), data)
}

// IniConfig 核心, 解析数据
func (ini *IniConfig) parseData(dir string, data []byte) (*IniConfigContainer, error) {
	cfg := &IniConfigContainer{
		data:           make(map[string]map[string]string),
		sectionComment: make(map[string]string),
		keyComment:     make(map[string]string),
		RWMutex:        sync.RWMutex{},
	}
	cfg.Lock()
	defer cfg.Unlock()

	var comment bytes.Buffer
	buf := bufio.NewReader(bytes.NewBuffer(data)) // 构建一个BufferReader, 方便行读取

	// 检查ini文件的BOM头
	head, err := buf.Peek(3)
	if err == nil && head[0] == 239 && head[1] == 187 && head[2] == 191 {
		for i := 1; i <= 3; i++ {
			buf.ReadByte()
		}
	}

	section := defaultSection
	for {
		line, _, err := buf.ReadLine()

		// 行末
		if err == io.EOF {
			break
		}

		// 其他未知错误
		if _, ok := err.(*os.PathError); ok {
			return nil, err
		}

		// 开始解析,
		// 第1步: 去空格, 判断是否为空行
		line = bytes.TrimSpace(line)
		if bytes.Equal(line, bEmpty) {
			continue
		}

		// 第2步: 注释行, 只会保存注释内容
		var bComment []byte
		switch {
		case bytes.HasPrefix(line, bNumComment): // #
			bComment = bNumComment
		case bytes.HasPrefix(line, bSemComment): // ;
			bComment = bSemComment
		}
		if bComment != nil {
			line = bytes.TrimLeft(line, string(bComment)) // 只是保存注释内容
			if comment.Len() > 0 { // 多行注释情况,需要手动添加换行符
				comment.WriteByte('\n')
			}
			comment.Write(line)
			continue
		}

		// 第3步: section行
		if bytes.HasPrefix(line, sectionStart) && bytes.HasSuffix(line, sectionEnd) {
			section = strings.ToLower(string(line[1: len(line)-1])) // 获取小写的section名称
			if comment.Len() > 0 {
				cfg.sectionComment[section] = comment.String() // 存储section注释
				comment.Reset()                                // 很关键, 重置为空
			}
			if _, ok := cfg.data[section]; !ok {
				cfg.data[section] = make(map[string]string) // 构建section当中的kv
			}
			continue
		}

		// 第4步: kv行
		if _, ok := cfg.data[section]; !ok { // section是default
			cfg.data[section] = make(map[string]string)
		}
		keyValue := bytes.SplitN(line, bEqual, 2)

		key := string(bytes.TrimSpace(keyValue[0]))
		key = strings.ToLower(key) // key全部是小写

		// 第5步: include行, 意味着引入新文件
		if len(keyValue) == 1 && strings.HasPrefix(key, "include") {
			includefiles := strings.Fields(key) // 获取字段[include, file, ...]
			if includefiles[0] == "include" && len(includefiles) == 2 {
				otherfile := strings.Trim(includefiles[1], `"`) // 去掉"
				if !filepath.IsAbs(otherfile) {
					otherfile = filepath.Join(dir, otherfile)
				}

				i, err := ini.parseFile(otherfile)
				if err != nil {
					return nil, err
				}

				// 数据保存和覆盖
				for sec, dt := range i.data {
					if _, ok := cfg.data[sec]; !ok {
						cfg.data[sec] = make(map[string]string)
					}
					for k, v := range dt {
						cfg.data[sec][k] = v
					}
				}

				for sec, comm := range i.sectionComment {
					cfg.sectionComment[sec] = comm
				}

				for k, comm := range i.keyComment {
					cfg.keyComment[k] = comm
				}

				continue
			}
		}

		// 第6步: 保存kv
		if len(keyValue) != 2 {
			return nil, errors.New("read the content error: \"" + string(line) + "\", should key = val")
		}
		val := bytes.TrimSpace(keyValue[1])
		if bytes.HasPrefix(val, bDQuote) {
			val = bytes.Trim(val, `"`)
		}

		cfg.data[section][key] = ExpandValueEnv(string(val))

		// 保存的是key注释, 即section.key; 注意: 如果只有section,没有key,这种注释不会被保存
		if comment.Len() > 0 {
			cfg.keyComment[section+"."+key] = comment.String()
			comment.Reset()
		}

	}
	return cfg, nil
}

// ParseData parse ini the data
// When include other.conf,other.conf is either absolute directory
// or under beego in default temporary directory(/tmp/beego[-username]).
func (ini *IniConfig) ParseData(data []byte) (Configer, error) {
	dir := "beego"
	currentUser, err := user.Current()
	if err == nil {
		dir = "beego-" + currentUser.Username
	}
	dir = filepath.Join(os.TempDir(), dir)
	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, err
	}

	return ini.parseData(dir, data)
}

//--------------------------------------------------------------------------------------------------

// 实现的 Configer接口
type IniConfigContainer struct {
	data           map[string]map[string]string // section => key:val (存储ini文件有效的内容)
	sectionComment map[string]string            // section : comment (存储的section的注释, 为了文件重新保存)
	keyComment     map[string]string            // key : comment (存储的是key的注释, 为了文件重新保存)
	sync.RWMutex
}

// Bool returns the boolean value for a given key.
func (c *IniConfigContainer) Bool(key string) (bool, error) {
	return ParseBool(c.getdata(key))
}

// DefaultBool returns the boolean value for a given key.
// if err != nil return defaultval
func (c *IniConfigContainer) DefaultBool(key string, defaultval bool) bool {
	v, err := c.Bool(key)
	if err != nil {
		return defaultval
	}
	return v
}

// Int returns the integer value for a given key.
func (c *IniConfigContainer) Int(key string) (int, error) {
	return strconv.Atoi(c.getdata(key))
}

// DefaultInt returns the integer value for a given key.
// if err != nil return defaultval
func (c *IniConfigContainer) DefaultInt(key string, defaultval int) int {
	v, err := c.Int(key)
	if err != nil {
		return defaultval
	}
	return v
}

// Int64 returns the int64 value for a given key.
func (c *IniConfigContainer) Int64(key string) (int64, error) {
	return strconv.ParseInt(c.getdata(key), 10, 64)
}

// DefaultInt64 returns the int64 value for a given key.
// if err != nil return defaultval
func (c *IniConfigContainer) DefaultInt64(key string, defaultval int64) int64 {
	v, err := c.Int64(key)
	if err != nil {
		return defaultval
	}
	return v
}

// Float returns the float value for a given key.
func (c *IniConfigContainer) Float(key string) (float64, error) {
	return strconv.ParseFloat(c.getdata(key), 64)
}

// DefaultFloat returns the float64 value for a given key.
// if err != nil return defaultval
func (c *IniConfigContainer) DefaultFloat(key string, defaultval float64) float64 {
	v, err := c.Float(key)
	if err != nil {
		return defaultval
	}
	return v
}

// String returns the string value for a given key.
func (c *IniConfigContainer) String(key string) string {
	return c.getdata(key)
}

// DefaultString returns the string value for a given key.
// if err != nil return defaultval
func (c *IniConfigContainer) DefaultString(key string, defaultval string) string {
	v := c.String(key)
	if v == "" {
		return defaultval
	}
	return v
}

// Strings returns the []string value for a given key.
// Return nil if config value does not exist or is empty.
func (c *IniConfigContainer) Strings(key string) []string {
	v := c.String(key)
	if v == "" {
		return nil
	}
	return strings.Split(v, ";")
}

// DefaultStrings returns the []string value for a given key.
// if err != nil return defaultval
func (c *IniConfigContainer) DefaultStrings(key string, defaultval []string) []string {
	v := c.Strings(key)
	if v == nil {
		return defaultval
	}
	return v
}

func (c *IniConfigContainer) GetSection(section string) (map[string]string, error) {
	if v, ok := c.data[section]; ok {
		return v, nil
	}
	return nil, errors.New("not exist section")
}

// 将 IniConfigContainer 内容保存为一个ini文件
// section注释 -> section -> key注释 -> k,v
func (c *IniConfigContainer) SaveConfigFile(filename string) (err error) {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// 获取注释, "section", "section.key"
	getCommentStr := func(section, key string) string {
		var (
			comment string
			ok      bool
		)
		if len(key) == 0 {
			comment, ok = c.sectionComment[section]
		} else {
			comment, ok = c.keyComment[section+"."+key]
		}

		// 对注释进行处理
		if ok {
			if len(comment) == 0 || len(strings.TrimSpace(comment)) == 0 {
				return string(bNumComment)
			}
			prefix := string(bNumComment)
			// 增加注释头 "#"
			return prefix + strings.Replace(comment, lineBreak, lineBreak+prefix, -1)
		}

		return ""
	}

	buf := bytes.NewBuffer(nil)
	// default section, 没有section注释, 必须先写default
	if dt, ok := c.data[defaultSection]; ok {
		for key, val := range dt {
			if key != " " {
				// 写入key的注释
				if v := getCommentStr(defaultSection, key); len(v) > 0 {
					if _, err = buf.WriteString(v + lineBreak); err != nil {
						return err
					}
				}

				// 写入k,v
				if _, err = buf.WriteString(key + string(bEqual) + val + lineBreak); err != nil {
					return err
				}
			}
		}

		// 换行
		if _, err = buf.WriteString(lineBreak); err != nil {
			return err
		}
	}

	// 自定义 section, 有section注释
	for section, dt := range c.data {
		if section != defaultSection {
			// 写入section注释
			if v := getCommentStr(section, ""); len(v) > 0 {
				if _, err = buf.WriteString(v + lineBreak); err != nil {
					return err
				}
			}

			// 写入section
			if _, err = buf.WriteString(string(sectionStart) + section + string(sectionEnd) + lineBreak); err != nil {
				return err
			}

			for key, val := range dt {
				if key != " " {
					// 写入key注释
					if v := getCommentStr(section, key); len(v) > 0 {
						if _, err = buf.WriteString(v + lineBreak); err != nil {
							return err
						}
					}

					// 写入k,v
					if _, err = buf.WriteString(key + string(bEqual) + val + lineBreak); err != nil {
						return err
					}
				}
			}

			// 换行
			if _, err = buf.WriteString(lineBreak); err != nil {
				return err
			}
		}
	}
	_, err = buf.WriteTo(f)
	return err
}

// 设置看k,v, 其中k支持 section::key的格式, section不存在则创建
func (c *IniConfigContainer) Set(key, value string) error {
	c.Lock()
	defer c.Unlock()
	if len(key) == 0 {
		return errors.New("key is empty")
	}

	var (
		section, k string
		sectionKey = strings.Split(strings.ToLower(key), "::")
	)

	if len(sectionKey) >= 2 {
		section = sectionKey[0]
		k = sectionKey[1]
	} else {
		section = defaultSection
		k = sectionKey[0]
	}

	if _, ok := c.data[section]; !ok {
		c.data[section] = make(map[string]string)
	}
	c.data[section][k] = value
	return nil
}

// 获取raw value
func (c *IniConfigContainer) DIY(key string) (v interface{}, err error) {
	if v, ok := c.data[strings.ToLower(key)]; ok {
		return v, nil
	}
	return v, errors.New("key not find")
}

// 获取 section::key 或者 key, 辅助函数
func (c *IniConfigContainer) getdata(key string) string {
	if len(key) == 0 {
		return ""
	}
	c.RLock()
	defer c.RUnlock()

	var (
		section, k string
		sectionKey = strings.Split(strings.ToLower(key), "::")
	)
	if len(sectionKey) >= 2 {
		section = sectionKey[0]
		k = sectionKey[1]
	} else {
		section = defaultSection
		k = sectionKey[0]
	}

	if v, ok := c.data[section]; ok {
		if vv, ok := v[k]; ok {
			return vv
		}
	}
	return ""
}

func init() {
	Register("ini", &IniConfig{})
}
