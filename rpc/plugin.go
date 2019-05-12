package main

import (
	"io/ioutil"
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
)

// 自定义的 netprc 插件
type netrpcPlugin struct {
	*generator.Generator
}

func (p *netrpcPlugin) Name() string {
	return "netrpc"
}

func (p *netrpcPlugin) Init(g *generator.Generator) {
	p.Generator = g
}

func (p *netrpcPlugin) GenerateImports(fd *generator.FileDescriptor) {
	if len(fd.Service) > 0 {
		p.genImportCode(fd)
	}
}

func (p *netrpcPlugin) Generate(fd *generator.FileDescriptor) {
	for _, sdp := range fd.Service {
		p.genServiceCode(sdp)
	}
}

func (p *netrpcPlugin) genImportCode(fd *generator.FileDescriptor) {
	p.P(`import "net/rpc"`)
}

// 描述服务的元信息
type ServiceSpec struct {
	ServiceName string
	MethodList  []ServiceMethodSpec
}

type ServiceMethodSpec struct {
	MethodName     string
	InputTypeName  string
	OutputTypeName string
}

func (p *netrpcPlugin) buildServiceSpec(sdp *descriptor.ServiceDescriptorProto) *ServiceSpec {
	spec := &ServiceSpec{
		ServiceName: generator.CamelCase(sdp.GetName()),
	}
	for _, m := range sdp.Method {
		spec.MethodList = append(spec.MethodList, ServiceMethodSpec{
			MethodName:generator.CamelCase(m.GetName()),
			InputTypeName:p.TypeName(p.ObjectNamed(m.GetInputType())),
			OutputTypeName:p.TypeName(p.ObjectNamed(m.GetOutputType())),
		})
	}
	return spec
}

func (p *netrpcPlugin) genServiceCode(sdp *descriptor.ServiceDescriptorProto) {
	p.P("// TODO:service code, Name = " + sdp.GetName())
}

func init() {
	generator.RegisterPlugin(new(netrpcPlugin))
}

// github.com/golang/protobuf/protoc-gen-go/main.go 函数的克隆版本
func grpc() {
	g := generator.New()
	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		g.Error(err, "reading input")
	}

	if err := proto.Unmarshal(data, g.Request); err != nil {
		g.Error(err, "parsing input proto")
	}

	if len(g.Request.FileToGenerate) == 0 {
		g.Fail("no files to generate")
	}

	g.CommandLineParameters(g.Request.GetParameter())

	g.WrapTypes()

	g.SetPackageNames()
	g.BuildTypeNameMap()
	g.GenerateAllFiles()

	data, err = proto.Marshal(g.Response)
	if err != nil {
		g.Error(err, "failed to marshal output proto")
	}

	_, err = os.Stdout.Write(data)
	if err != nil {
		g.Error(err, "failed to write output proto")
	}
}

func main() {

}

// 插件使用:
// 将插件编译成 protoc-gen-go-netrpc, 然后放到系统的 $PATH 的目录下
// protoc --go-netrpc_out=plugins=netrpc:. hello.proto
//
// 效果:
// 在生成的代码当中有 netrpc 当中添加的代码
