package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/get", func(writer http.ResponseWriter, request *http.Request) {
		fmt.Printf("%+v\n", request)
		writer.Write([]byte("Hello World"))
	})
	http.ListenAndServe("127.0.0.1:8000", nil)
}
