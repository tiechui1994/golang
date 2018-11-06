package app

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/astaxie/beego/logs"
	"github.com/astaxie/beego/utils"
)

// 前端模板
var (
	beegoTplFuncMap           = make(template.FuncMap)
	beeViewPathTemplateLocked = false
	beeViewPathTemplates      = make(map[string]map[string]*template.Template) // 缓存模板, viewPath -> [filename -> Template]
	templatesLock             sync.RWMutex
	beeTemplateExt            = []string{"tpl", "html"}           // 模板的扩展名
	beeTemplateEngines        = map[string]templatePreProcessor{} // 存储的是扩展名对应的解析函数, extension -> preprocessor
)

func init() {
	beegoTplFuncMap["dateformat"] = DateFormat
	beegoTplFuncMap["date"] = Date
	beegoTplFuncMap["compare"] = Compare
	beegoTplFuncMap["compare_not"] = CompareNot
	beegoTplFuncMap["not_nil"] = NotNil
	beegoTplFuncMap["not_null"] = NotNil
	beegoTplFuncMap["substr"] = Substr
	beegoTplFuncMap["html2str"] = HTML2str
	beegoTplFuncMap["str2html"] = Str2html
	beegoTplFuncMap["htmlquote"] = Htmlquote
	beegoTplFuncMap["htmlunquote"] = Htmlunquote
	beegoTplFuncMap["renderform"] = RenderForm
	beegoTplFuncMap["assets_js"] = AssetsJs
	beegoTplFuncMap["assets_css"] = AssetsCSS
	beegoTplFuncMap["config"] = GetConfig
	beegoTplFuncMap["map_get"] = MapGet

	// Comparisons
	beegoTplFuncMap["eq"] = eq // ==
	beegoTplFuncMap["ge"] = ge // >=
	beegoTplFuncMap["gt"] = gt // >
	beegoTplFuncMap["le"] = le // <=
	beegoTplFuncMap["lt"] = lt // <
	beegoTplFuncMap["ne"] = ne // !=

	beegoTplFuncMap["urlfor"] = URLFor // build a URL to match a Controller and it's method
}

//---------------------------------------------------------------------------------------------------------------

// 注册函数:
// 此方法必须在 beego.Run() 之前调用, 否则可能产生panic
func AddViewPath(viewPath string) error {
	if beeViewPathTemplateLocked { // 在beego.Run()之前设置
		if _, exist := beeViewPathTemplates[viewPath]; exist {
			return nil
		}
		panic("Can not add new view paths after beego.Run()")
	}
	beeViewPathTemplates[viewPath] = make(map[string]*template.Template)
	return BuildTemplate(viewPath)
}

// beego.Run() 之前调用.
func lockViewPaths() {
	beeViewPathTemplateLocked = true
}

//---------------------------------------------------------------------------------------------------------------

type templatePreProcessor func(root, path string, funcs template.FuncMap) (*template.Template, error)

type templateFile struct {
	root  string
	files map[string][]string // dir -> filePath
}

/**
将paths分成两部分, 第一部分是subDir(不包含tf.root), 第二部分是full path(不包含tf.root)
例如: tf.root="views",
	paths是"views/errors/404.html", subDir为"errors", full path是"errors/404.html"
	paths是"views/admin/errors/404.html", subDir是"admin/errors", full path是"admin/errors/404.html"
*/
func (tf *templateFile) visit(paths string, f os.FileInfo, err error) error {
	if f == nil {
		return err
	}
	if f.IsDir() || (f.Mode()&os.ModeSymlink) > 0 {
		return nil
	}
	if !HasTemplateExt(paths) {
		return nil
	}

	replace := strings.NewReplacer("\\", "/") // 将所有的"\\"使用"/"替换
	file := strings.TrimLeft(replace.Replace(paths[len(tf.root):]), "/")
	subDir := filepath.Dir(file)

	tf.files[subDir] = append(tf.files[subDir], file)
	return nil
}

// 模板解析
func ExecuteTemplate(wr io.Writer, name string, data interface{}) error {
	return ExecuteViewPathTemplate(wr, name, BConfig.WebConfig.ViewsPath, data)
}

func ExecuteViewPathTemplate(wr io.Writer, name string, viewPath string, data interface{}) error {
	if BConfig.RunMode == DEV {
		templatesLock.RLock()
		defer templatesLock.RUnlock()
	}

	if beeTemplates, ok := beeViewPathTemplates[viewPath]; ok {
		if t, ok := beeTemplates[name]; ok {
			var err error
			if t.Lookup(name) != nil { // 模板查找
				err = t.ExecuteTemplate(wr, name, data) // 特定模板渲染
			} else {
				err = t.Execute(wr, data)
			}
			if err != nil {
				logs.Trace("template Execute err:", err)
			}
			return err
		}
		panic("can't find templatefile in the path:" + viewPath + "/" + name)
	}
	panic("Unknown view path:" + viewPath)
}

// 用户注册模板函数
func AddFuncMap(key string, fn interface{}) error {
	beegoTplFuncMap[key] = fn
	return nil
}

// 检测文件的扩展名
func HasTemplateExt(paths string) bool {
	for _, v := range beeTemplateExt {
		if strings.HasSuffix(paths, "."+v) {
			return true
		}
	}
	return false
}

// 增加文件扩展
func AddTemplateExt(ext string) {
	for _, v := range beeTemplateExt {
		if v == ext {
			return
		}
	}
	beeTemplateExt = append(beeTemplateExt, ext)
}

// build 给定目录下模板文件
// BuildTemplate will build all template files in a directory.
// it makes beego can render any template file in view directory.
func BuildTemplate(dir string, files ...string) error {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.New("dir open err") // 目录不存在
	}

	beeTemplates, ok := beeViewPathTemplates[dir]
	if !ok {
		panic("Unknown view path: " + dir)
	}

	self := &templateFile{
		root:  dir,
		files: make(map[string][]string),
	}
	// 针对dir目录下的每个文件执行visit函数, 初始化files
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		return self.visit(path, f, err)
	})
	if err != nil {
		fmt.Printf("filepath.Walk() returned %v\n", err)
		return err
	}

	// build, 如果files为空,则build所有的模板, 否则只build files当中的模板
	buildAllFiles := len(files) == 0
	for _, v := range self.files {
		for _, file := range v {
			if buildAllFiles || utils.InSlice(file, files) {
				templatesLock.Lock()
				ext := filepath.Ext(file)
				var t *template.Template
				if len(ext) == 0 { // 没有扩展名
					t, err = getTemplate(self.root, file, v...)
				} else if fn, ok := beeTemplateEngines[ext[1:]]; ok { // 扩展名存在beeTemplateEngines当中, 使用专用引擎解析
					t, err = fn(self.root, file, beegoTplFuncMap)
				} else { // 新扩展名
					t, err = getTemplate(self.root, file, v...)
				}

				if err != nil {
					logs.Error("parse template err:", file, err)
					templatesLock.Unlock()
					return err
				}

				beeTemplates[file] = t
				templatesLock.Unlock()
			}
		}
	}
	return nil
}

/**
root: root目录
file: 文件路径
parent: 可选选项
t: 模板
*/
func getTplDeep(root, file, parent string, t *template.Template) (*template.Template, [][]string, error) {
	var (
		fileAbsPath string
		rParent     string
	)

	if strings.HasPrefix(file, "../") {
		rParent = filepath.Join(filepath.Dir(parent), file)
		fileAbsPath = filepath.Join(root, filepath.Dir(parent), file)
	} else {
		rParent = file
		fileAbsPath = filepath.Join(root, file)
	}

	if e := utils.FileExists(fileAbsPath); !e {
		panic("can't find template file:" + file)
	}

	data, err := ioutil.ReadFile(fileAbsPath)
	if err != nil {
		return nil, [][]string{}, err
	}
	t, err = t.New(file).Parse(string(data))
	if err != nil {
		return nil, [][]string{}, err
	}
	reg := regexp.MustCompile(BConfig.WebConfig.TemplateLeft + "[ ]*template[ ]+\"([^\"]+)\"")
	allSub := reg.FindAllStringSubmatch(string(data), -1)
	for _, m := range allSub {
		if len(m) == 2 {
			tl := t.Lookup(m[1])
			if tl != nil {
				continue
			}
			if !HasTemplateExt(m[1]) {
				continue
			}
			_, _, err = getTplDeep(root, m[1], rParent, t)
			if err != nil {
				return nil, [][]string{}, err
			}
		}
	}
	return t, allSub, nil
}

// 通过root, file构建一个Template
func getTemplate(root, file string, others ...string) (t *template.Template, err error) {
	t = template.New(file).Delims(BConfig.WebConfig.TemplateLeft, BConfig.WebConfig.TemplateRight).Funcs(beegoTplFuncMap)
	var subMods [][]string
	t, subMods, err = getTplDeep(root, file, "", t)
	if err != nil {
		return nil, err
	}
	t, err = _getTemplate(t, root, subMods, others...)

	if err != nil {
		return nil, err
	}
	return
}

func _getTemplate(t0 *template.Template, root string, subMods [][]string, others ...string) (t *template.Template, err error) {
	t = t0
	for _, m := range subMods {
		if len(m) == 2 {
			tpl := t.Lookup(m[1])
			if tpl != nil {
				continue
			}
			//first check filename
			for _, otherFile := range others {
				if otherFile == m[1] {
					var subMods1 [][]string
					t, subMods1, err = getTplDeep(root, otherFile, "", t)
					if err != nil {
						logs.Trace("template parse file err:", err)
					} else if len(subMods1) > 0 {
						t, err = _getTemplate(t, root, subMods1, others...)
					}
					break
				}
			}
			//second check define
			for _, otherFile := range others {
				var data []byte
				fileAbsPath := filepath.Join(root, otherFile)
				data, err = ioutil.ReadFile(fileAbsPath)
				if err != nil {
					continue
				}
				reg := regexp.MustCompile(BConfig.WebConfig.TemplateLeft + "[ ]*define[ ]+\"([^\"]+)\"")
				allSub := reg.FindAllStringSubmatch(string(data), -1)
				for _, sub := range allSub {
					if len(sub) == 2 && sub[1] == m[1] {
						var subMods1 [][]string
						t, subMods1, err = getTplDeep(root, otherFile, "", t)
						if err != nil {
							logs.Trace("template parse file err:", err)
						} else if len(subMods1) > 0 {
							t, err = _getTemplate(t, root, subMods1, others...)
						}
						break
					}
				}
			}
		}

	}
	return
}

// SetViewsPath sets view directory path in beego application.
func SetViewsPath(path string) *App {
	BConfig.WebConfig.ViewsPath = path
	return BeeApp
}

// SetStaticPath sets static directory path and proper url pattern in beego application.
// if beego.SetStaticPath("static","public"), visit /static/* to load static file in folder "public".
func SetStaticPath(url string, path string) *App {
	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}
	if url != "/" {
		url = strings.TrimRight(url, "/")
	}
	BConfig.WebConfig.StaticDir[url] = path
	return BeeApp
}

// DelStaticPath removes the static folder setting in this url pattern in beego application.
func DelStaticPath(url string) *App {
	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}
	if url != "/" {
		url = strings.TrimRight(url, "/")
	}
	delete(BConfig.WebConfig.StaticDir, url)
	return BeeApp
}

// AddTemplateEngine add a new templatePreProcessor which support extension
func AddTemplateEngine(extension string, fn templatePreProcessor) *App {
	AddTemplateExt(extension)
	beeTemplateEngines[extension] = fn
	return BeeApp
}
