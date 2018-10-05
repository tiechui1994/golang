package session

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
)

var (
	filepder      = &FileProvider{}
	gcmaxlifetime int64
)

// Session的存储结构
type FileSessionStore struct {
	sid    string
	lock   sync.RWMutex
	values map[interface{}]interface{} // 内存存储
}

func (fs *FileSessionStore) Set(key, value interface{}) error {
	fs.lock.Lock()
	defer fs.lock.Unlock()
	fs.values[key] = value
	return nil
}

func (fs *FileSessionStore) Get(key interface{}) interface{} {
	fs.lock.RLock()
	defer fs.lock.RUnlock()
	if v, ok := fs.values[key]; ok {
		return v
	}
	return nil
}

func (fs *FileSessionStore) Delete(key interface{}) error {
	fs.lock.Lock()
	defer fs.lock.Unlock()
	delete(fs.values, key)
	return nil
}

func (fs *FileSessionStore) Flush() error {
	fs.lock.Lock()
	defer fs.lock.Unlock()
	fs.values = make(map[interface{}]interface{})
	return nil
}

func (fs *FileSessionStore) SessionID() string {
	return fs.sid
}

// 将Session的内容以Gob流的形式写入本地文件
func (fs *FileSessionStore) SessionRelease(w http.ResponseWriter) {
	filepder.lock.Lock()
	defer filepder.lock.Unlock()
	b, err := EncodeGob(fs.values) // 将Session存储的内容编码成Gob流
	if err != nil {
		SLogger.Println(err)
		return
	}
	// 文件路径: /savePath/SessionID[0]/SessionID[1]/SessionID
	_, err = os.Stat(path.Join(filepder.savePath, string(fs.sid[0]), string(fs.sid[1]), fs.sid))
	var f *os.File
	if err == nil { //文件存在
		f, err = os.OpenFile(path.Join(filepder.savePath, string(fs.sid[0]), string(fs.sid[1]), fs.sid), os.O_RDWR, 0777)
		if err != nil {
			SLogger.Println(err)
			return
		}
	} else if os.IsNotExist(err) { //文件不存在
		f, err = os.Create(path.Join(filepder.savePath, string(fs.sid[0]), string(fs.sid[1]), fs.sid))
		if err != nil {
			SLogger.Println(err)
			return
		}
	} else {
		return //未知错误
	}
	f.Truncate(0) // 文件大小修改为0, 不会移动文件指针
	f.Seek(0, 0)  // 指针移动
	f.Write(b)
	f.Close()
}

// 文件引擎, 配置参数savePath
type FileProvider struct {
	lock        sync.RWMutex
	maxlifetime int64
	savePath    string
}

func (fp *FileProvider) SessionInit(maxlifetime int64, savePath string) error {
	fp.maxlifetime = maxlifetime
	fp.savePath = savePath
	return nil
}

// 根据SessionId读取Session的存储实例
func (fp *FileProvider) SessionRead(sid string) (Store, error) {
	filepder.lock.Lock()
	defer filepder.lock.Unlock()

	err := os.MkdirAll(path.Join(fp.savePath, string(sid[0]), string(sid[1])), 0777) // 创建目录结构
	if err != nil {
		SLogger.Println(err.Error())
	}
	// 与文件写入判断类似
	_, err = os.Stat(path.Join(fp.savePath, string(sid[0]), string(sid[1]), sid))
	var f *os.File
	if err == nil {
		f, err = os.OpenFile(path.Join(fp.savePath, string(sid[0]), string(sid[1]), sid), os.O_RDWR, 0777)
	} else if os.IsNotExist(err) {
		f, err = os.Create(path.Join(fp.savePath, string(sid[0]), string(sid[1]), sid))
	} else {
		return nil, err
	}

	defer f.Close()

	// 修改文件的访问和修改时间
	os.Chtimes(path.Join(fp.savePath, string(sid[0]), string(sid[1]), sid), time.Now(), time.Now())
	var kv map[interface{}]interface{}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 { //创建的新文件
		kv = make(map[interface{}]interface{})
	} else { // 已存在的文件
		kv, err = DecodeGob(b)
		if err != nil {
			return nil, err
		}
	}

	ss := &FileSessionStore{sid: sid, values: kv}
	return ss, nil
}

// SessionExist 检查SessionID对应的文件是否存在
// it checks the file named from sid exist or not.
func (fp *FileProvider) SessionExist(sid string) bool {
	filepder.lock.Lock()
	defer filepder.lock.Unlock()

	_, err := os.Stat(path.Join(fp.savePath, string(sid[0]), string(sid[1]), sid))
	return err == nil
}

// SessionDestroy 删除Session的存储内容
func (fp *FileProvider) SessionDestroy(sid string) error {
	filepder.lock.Lock()
	defer filepder.lock.Unlock()
	os.Remove(path.Join(fp.savePath, string(sid[0]), string(sid[1]), sid))
	return nil
}

// GC过程
func (fp *FileProvider) SessionGC() {
	filepder.lock.Lock()
	defer filepder.lock.Unlock()

	gcmaxlifetime = fp.maxlifetime
	filepath.Walk(fp.savePath, gcpath)
}

// 获取活跃的SessionID的个数 --> 统计已经存储的Session的文件个数
func (fp *FileProvider) SessionAll() int {
	a := &activeSession{}
	err := filepath.Walk(fp.savePath, func(path string, f os.FileInfo, err error) error {
		return a.visit(path, f, err)
	})
	if err != nil {
		SLogger.Printf("filepath.Walk() returned %v\n", err)
		return 0
	}
	return a.total
}

// 为Session重新生成一个ID, 删除old文件 并且从新id生成一个新文件
func (fp *FileProvider) SessionRegenerate(oldsid, sid string) (Store, error) {
	filepder.lock.Lock()
	defer filepder.lock.Unlock()

	oldPath := path.Join(fp.savePath, string(oldsid[0]), string(oldsid[1]))
	oldSidFile := path.Join(oldPath, oldsid)
	newPath := path.Join(fp.savePath, string(sid[0]), string(sid[1]))
	newSidFile := path.Join(newPath, sid)

	// new sid file is exist
	_, err := os.Stat(newSidFile)
	if err == nil {
		return nil, fmt.Errorf("newsid %s exist", newSidFile)
	}

	err = os.MkdirAll(newPath, 0777)
	if err != nil {
		SLogger.Println(err.Error())
	}

	// old 文件存在
	// 1.read and parse file content
	// 2.write content to new sid file
	// 3.remove old sid file, change new sid file atime and ctime
	// 4.return FileSessionStore
	_, err = os.Stat(oldSidFile)
	if err == nil {
		b, err := ioutil.ReadFile(oldSidFile)
		if err != nil {
			return nil, err
		}

		var kv map[interface{}]interface{}
		if len(b) == 0 {
			kv = make(map[interface{}]interface{})
		} else {
			kv, err = DecodeGob(b)
			if err != nil {
				return nil, err
			}
		}

		ioutil.WriteFile(newSidFile, b, 0777)
		os.Remove(oldSidFile)
		os.Chtimes(newSidFile, time.Now(), time.Now())
		ss := &FileSessionStore{sid: sid, values: kv}
		return ss, nil
	}

	// old文件不存在
	newf, err := os.Create(newSidFile)
	if err != nil {
		return nil, err
	}
	newf.Close()
	ss := &FileSessionStore{sid: sid, values: make(map[interface{}]interface{})}
	return ss, nil
}

// 定时任务的工作(gc)
func gcpath(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	if (info.ModTime().Unix() + gcmaxlifetime) < time.Now().Unix() {
		os.Remove(path)
	}
	return nil
}

type activeSession struct {
	total int
}

func (as *activeSession) visit(paths string, f os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if f.IsDir() {
		return nil
	}
	as.total = as.total + 1
	return nil
}

func init() {
	Register("file", filepder)
}
