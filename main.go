package main

import (
	"net/http"
	"sync"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /kv/{key}", db.put)       // 存入
	mux.HandleFunc("GET /kv/{key}", db.get)       // 读取
	mux.HandleFunc("DELETE /kv/{key}", db.delete) // 删除
	mux.HandleFunc("GET /kv", db.list)            // 列出所有 key
	http.ListenAndServe(":8080", mux)             // 启动服务：监听 8080 端口一直跑，nil = 用全局路由表（Ctrl+C 才停）
}

type store struct { // 存储：数据 data + 保护它的锁 mu，绑成一套
	mu   sync.RWMutex      // 读写锁：保护下面的 data（读用 RLock、写用 Lock）
	data map[string]string // 数据本体：key → value，都存纯文本
}

var db = store{data: map[string]string{}} // 全局唯一的存储实例；data 必须初始化（nil map 写入会 panic）

func (s *store) put(w http.ResponseWriter, r *http.Request) {

}

func (s *store) get(w http.ResponseWriter, r *http.Request) {

}

func (s *store) delete(w http.ResponseWriter, r *http.Request) {

}

func (s *store) list(w http.ResponseWriter, r *http.Request) {

}
