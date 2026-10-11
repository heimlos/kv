package main

import (
	"net/http"
)

func main() {
	http.HandleFunc("/", index)       // 登记路由：访问 "/" 时交给 index 处理
	http.ListenAndServe(":8080", nil) // 启动服务：监听 8080 端口一直跑，nil = 用全局路由表（Ctrl+C 才停）
}
func index(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}
