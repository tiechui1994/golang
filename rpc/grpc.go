package main

/**
protobuf 与 rpc:
  Protobuf 核心工具集是C++语言开发的, 在官方的 protoc 编译器中并不支持 Go 语言. 要想基于`.proto`
文件生成相应的 Go 语言代码, 需要安装相应的插件.

  首先安装官方的 protoc 工具. 可以从 https://github.com/google/protobuf/releases 下载. 然后
是安装针对 Go 语言的代码生成插件. 可以通过 go get github.com/golang/protobuf/protoc-gen-go 安
装.

  生成 Go 代码: protoc --go_out=. hello.proto
  其中 go_out 参数告知 protoc 编译器去加载对应的 protoc-gen-go 工具, 然后通过该工具生成代码, 生成
代码的目录放在当前目录.

RPC格式介绍:

  SayHello(request Request, reponse *Response) error

  生成的方法是围绕 Request 和 Response 类型展开的. Request是 RPC 请求的参数, Response 是 RPC 的
返回相应. 其中 Request 和 Response 的 ProtoMessage 方法表示这是一个实现了 proto.Message 接口的方
法.


 在 protoc-gen-go 内部已经集成了一个名为grpc的插件, 可以针对 gRPC 生成代码:
 protoc --go_out=plugins=grpc:.  hello.proto

 在生成的代码中增加 XxxServer 和 XxxClient 的新类型. 这些类型是为 gRPC 服务的.
**/

/**
定制代码生成插件:

**/