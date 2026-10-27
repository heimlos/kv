package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

func main() {
	f, err := os.OpenFile("data.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644) // 打开日志文件：不存在就创建，写入模式，追加写入，权限 0644
	if err != nil {
		panic(err) // 打开失败就 panic（直接崩溃，打印错因）
	}

	db := &store{data: map[string]string{}, log: f} // 存储实例：在 main 里建（局部变量）；data 必须初始化（nil map 写入会 panic）

	err = db.replay("data.log") // 启动时把日志重放回内存

	if err != nil {
		panic(err)
	}

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

type putEntry struct { // 日志条目：存入操作的记录
	Op    string `json:"op"`    // 操作：put
	Key   string `json:"key"`   // key
	Value string `json:"value"` // value
}

type delEntry struct { // 日志条目：删除操作的记录
	Op  string `json:"op"`  // 操作：delete
	Key string `json:"key"` // key
}

type logEntry struct { // 日志行：put / del 共用这一个结构体解析
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value"` // del 行没这个字段 → 解出来是空串
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
	line, err := json.Marshal(putEntry{Op: "put", Key: key, Value: s.data[key]})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError) // 依次：w 往哪写 → err.Error()（错因）→ 500 状态码
		return
	}

	_, err = s.log.Write(append(line, '\n')) // 写入日志：末尾加换行符
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError) // 依次：w 往哪写 → err.Error()（错因）→ 500 状态码
		return
	}

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
	line, err := json.Marshal(delEntry{Op: "del", Key: key})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError) // 依次：w 往哪写 → err.Error()（错因）→ 500 状态码
		return
	}

	_, err = s.log.Write(append(line, '\n')) // 写入日志：末尾加换行符
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError) // 依次：w 往哪写 → err.Error()（错因）→ 500 状态码
		return
	}

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

func (s *store) replay(path string) error {
	f, err := os.Open(path) // 只读打开日志：os.Open = OpenFile(path, O_RDONLY, 0) 的简写；不能用 s.log（那是 V1 的 O_WRONLY 句柄，读不了）
	if err != nil {
		return err
	}

	defer f.Close()
	b, err := io.ReadAll(f) // 把整个文件一次读进内存：b 是全部内容（[]byte，日志文件小所以能这么干），err 是读失败的原因
	if err != nil {
		return err
	}

	lines := strings.Split(string(b), "\n") // 按换行切成一片，一个元素就是一行；文件以 \n 结尾 → 最后会多出一个空串（104 行切成 105 个）
	lastIdx := -1                           // 最后一条非空行的下标（-1 = 还没有）
	for i, ln := range lines {
		if strings.TrimSpace(ln) != "" { // 去掉两端空白后还有内容 = 真行；空串 / 只有空格 / 只剩 \r 都算空行，不算
			lastIdx = i
		}
	}
	pos := 0 // 当前行在文件里的起始字节
	for i, ln := range lines {

		lineStart := pos   // 本行的第一个字节：pos 里已经加过上一行的换行符，所以不会指到 \n 上
		pos += len(ln) + 1 // 先记本行再往后推：len(ln) 不含换行符（Split 把 \n 吃掉了），所以 +1 补回来

		if strings.TrimSpace(ln) == "" {
			continue
		}
		var e logEntry
		err := json.Unmarshal([]byte(ln), &e) // 把一行 JSON 解进 e：① string 先转 []byte ② 传 &e（地址）才能写进去
		if err != nil {
			// 坏行①：JSON 解析失败 —— 只有"最后一行"才算崩溃尾巴（忽略），否则是真损坏（报错）
			if i == lastIdx {
				fmt.Println("忽略尾部坏行:", ln)
				err := os.Truncate(path, int64(lineStart)) // 把半行切掉，否则下次写入会粘成中间坏行
				if err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("第 %d 行解析失败: %v", i+1, err)
		}

		switch e.Op {
		case "put":
			s.data[e.Key] = e.Value
		case "del":
			delete(s.data, e.Key)
		default:
			// 坏行②：op 不认识 —— 判断和上面一样，只看它是不是最后一行
			if i == lastIdx {
				fmt.Println("忽略尾部坏行:", ln)
				err := os.Truncate(path, int64(lineStart))
				if err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("第 %d 行 op 不认识: %q", i+1, e.Op)
		}
	}

	return nil
}
