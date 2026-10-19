package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"
)

func main() {
	f, err := os.OpenFile("data.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644) // 打开日志文件：不存在就创建，写入模式，追加写入，权限 0644
	if err != nil {
		panic(err) // 打开失败就 panic（直接崩溃，打印错因）
	}
	db := &store{data: map[string]string{}, log: f} // 存储实例：在 main 里建（局部变量）；data 必须初始化（nil map 写入会 panic）
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
	log  *os.File          // 日志文件句柄（一个打开着的文件，这里放 data.log）
}

func (s *store) put(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		// http.Error(往哪写, 写什么, 状态码)
		//   ① w                     → 往哪写：响应
		//   ② err.Error()           → 写什么：错因那句话（error 要 .Error() 转成 string）
		//   ③ http.StatusBadRequest → 状态码：常量，值就是 400（"客户端请求有问题"）
		http.Error(w, err.Error(), http.StatusBadRequest) // 依次：w 往哪写 → err.Error()（错因）→ 400 状态码
		return                                            // 提前返回：正文没读到，别再往下走
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = string(body)
	w.Write([]byte("ok"))
}

func (s *store) get(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.data[key]
	if !ok {
		http.Error(w, "not found", http.StatusNotFound) // 依次：w → "not found" → 404 状态码
		return
	}
	w.Write([]byte(value))
}

func (s *store) delete(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[key]
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	delete(s.data, key)
	w.Write([]byte("ok"))
}

func (s *store) list(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data)) // ② 空切片（不是 nil）+ 预分配
	for k := range s.data {                //    只取 key，不要 value
		keys = append(keys, k)
	}
	data, err := json.Marshal(keys) // ③ 编码成 JSON 文本：[]string → []byte（结果如 ["name","age"]）
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError) // 依次：w 往哪写 → err.Error()（错因）→ 500 状态码
		return
	}
	w.Write(data)
}
