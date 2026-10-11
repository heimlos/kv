package main

import (
	"net/http"
	"sync"
)

func main() {
	http.HandleFunc("/", index)       // 登记路由：访问 "/" 时交给 index 处理
	http.ListenAndServe(":8080", nil) // 启动服务：监听 8080 端口一直跑，nil = 用全局路由表（Ctrl+C 才停）
}

func index(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

type store struct { // 存储：数据 data + 保护它的锁 mu，绑成一套
	mu   sync.RWMutex      // 读写锁：保护下面的 data（读用 RLock、写用 Lock）
	data map[string]string // 数据本体：key → value，都存纯文本
}

var db = store{data: map[string]string{}} // 全局唯一的存储实例；data 必须初始化（nil map 写入会 panic）
