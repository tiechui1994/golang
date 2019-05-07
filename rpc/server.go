package main

import (
	"net/rpc"
	"net"
	"log"
	"fmt"

	"github.com/tiechui1994/golang/rpc/service"
)

type Error struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func NewError(code int, param ...interface{}) *Error {
	err := &Error{
		Code: code,
		Msg:  "unkown error",
	}

	if len(param) == 0 {
		return err
	}

	if len(param) > 0 {
		switch param[0].(type) {
		case string:
			err.Msg = param[0].(string)
		case error:
			err.Msg = param[0].(error).Error()
		}
	}

	return err
}

func (e *Error) Error() string {
	return fmt.Sprintf("code:%v, msg:%v", e.Code, e.Msg)
}

// ----------------- native rpc service ---------------------------
type NativeHello struct {
}

func (p *NativeHello) SayHello(request string, reponse *string) error {
	if request == "" {
		return NewError(-1, "the request is null")
	}

	*reponse = "Hello: " + request

	return nil
}

func RegisterNative() {
	rpc.RegisterName("NativeHello", new(NativeHello))
}

// ----------------- protobuf rpc service ---------------------------
// proto: hello.proto
// protoc --go_out=./service hello.proto // no plugin
// protoc --go_out=plugins=grpc:./service hello.proto // use plugin
//
// no plugin depend:
// 	github.com/golang/protobuf
//
// plugin depend:
// 	github.com/golang/protobuf
// 	google.golang.org/grpc -> github.com/grpc/grpc-go
// 	google.golang.org/genproto -> github.com/google/go-genproto
// 	golang.org/x/net -> github/golang/net
type ProtoHello struct {
}

func (p *ProtoHello) SayHello(request service.Request, response *service.Response) error {
	fmt.Println(request.String())

	response.Msg = "OK"
	response.Info = fmt.Sprintf("receive uid:%v", request.Uid)

	return nil
}

func RegisterProto() {
	rpc.RegisterName("ProtoHello", new(ProtoHello))
}

func main() {
	RegisterNative()
	RegisterProto()

	listener, err := net.Listen("tcp", ":1234")
	if err != nil {
		log.Fatal("ListenTCP error:", err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal("Accept error:", err)
		}

		// json decode
		// go rpc.ServeCodec(jsonrpc.NewServerCodec(conn))

		// gob decode
		go rpc.ServeConn(conn)
	}

}
