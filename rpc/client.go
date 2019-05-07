package main

import (
	"net/rpc"
	"fmt"
	"log"
	"time"

	"github.com/tiechui1994/golang/rpc/service"
)

func main() {
	// json encode
	// client, err := jsonrpc.Dial("tcp", "localhost:1234")

	// gob encode
	client, err := rpc.Dial("tcp", "localhost:1234")
	if err != nil {
		log.Fatal("dialing:", err)
	}

	var (
		res string
		req = fmt.Sprintf("%v", time.Now().UnixNano())
	)
	err = client.Call("NativeHello.SayHello", req, &res)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("native rpc: %+v \n", res)

	var (
		response service.Response
		request  = service.Request{
			Uid: fmt.Sprintf("%v", time.Now().UnixNano()),
		}
	)

	err = client.Call("ProtoHello.SayHello", request, &response)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("proto rpc: %+v \n", response)
}
