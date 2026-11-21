// devtools 静态文件服务器：本地调试前端用（go run ./devtools）。
package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	dir := flag.String("dir", "../frontend/dist", "静态目录")
	addr := flag.String("addr", "127.0.0.1:8642", "监听地址")
	flag.Parse()
	log.Printf("serving %s at http://%s", *dir, *addr)
	log.Fatal(http.ListenAndServe(*addr, http.FileServer(http.Dir(*dir))))
}
