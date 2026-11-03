# kv

用 Go 写的极简 KV 存储：HTTP 接口 + 内存 map + 追加日志（`data.log`）持久化，启动时 `replay` 恢复。

## 接口

| 方法 | 路径 | 作用 |
| --- | --- | --- |
| PUT | `/kv/{key}` | 存入（请求体就是 value） |
| GET | `/kv/{key}` | 读取 |
| DELETE | `/kv/{key}` | 删除 |
| GET | `/kv` | 列出所有 key（JSON 数组） |

```bash
curl -X PUT localhost:8080/kv/name -d "noetia"   # → ok
curl localhost:8080/kv/name                      # → noetia
curl localhost:8080/kv                           # → ["name"]
curl -X DELETE localhost:8080/kv/name            # → ok
curl -i localhost:8080/kv/name                   # → 404 not found
```

## 本地运行

```bash
go run .                                          # 直接跑
go build -o kv-server . && ./kv-server            # 编成二进制再跑
```

## 设计说明

**为什么 append-only**：每次写只往 `data.log` 末尾**追加**一行，从不改旧内容。好处是写入变成顺序追加，不会把已经写好的数据改坏；崩溃最多影响最后一行，而「最后一行不完整」很容易识别。

**日志格式**：每行一个 JSON，`json.Marshal` 生成后补 `\n`。

```text
{"op":"put","key":"name","value":"noetia"}
{"op":"del","key":"name"}
```

**重启怎么恢复**：`replay()` 在 `main` 里、监听端口之前跑——整个文件读进内存 → 按 `\n` 切行 → 逐行 `json.Unmarshal` → 按 `op` 依次 `put` / `del` 重建 map。空行跳过；`del` 行没有 `value`，解出来是空串。

**坏行策略**（对应「崩在写一半」）：解析失败或 `op` 不认识时，只容忍**最后一行**——打印提示并用 `os.Truncate` 按这行的起始偏移把半行切掉（否则下次追加会粘在坏行后面）；中间行坏了说明文件真损坏，直接报错退出（`panic`）。

**并发**：`sync.RWMutex` 保护 map，读用 `RLock`、写用 `Lock`，写日志也在锁内，保证 map 与 `data.log` 顺序一致。

## 已知限制

- 单机单进程：没有集群、复制、分片
- **无鉴权**：任何能访问 8080 的人都能读写删
- 无 TTL：key 不会自动过期
- 无快照 / 压缩：`data.log` 只增不减，恢复时整个文件读进内存
- 没有 `fsync`：进程崩溃时 OS 缓存里的最后几次写可能丢
- HTTP 明文，没有 TLS

## 部署（Ubuntu 24.04，已实测）

1. 静态编译（不依赖服务器 glibc）：

```bash
CGO_ENABLED=0 go build -o kv-server .
file kv-server     # ELF 64-bit, x86-64, statically linked
```

2. 上传并改属主 / 执行位：

```bash
scp kv-server root@<IP>:/home/kv/      # 本地执行
```

```bash
chown kv:kv /home/kv/kv-server         # 服务器上执行
chmod +x /home/kv/kv-server
```

3. 先手动跑一遍，排掉二进制 / 权限 / 端口问题：

```bash
su - kv -c 'cd /home/kv && ./kv-server'
```

4. 交给 systemd：`/etc/systemd/system/kv.service`

```ini
[Unit]
Description=KV store service
After=network.target

[Service]
User=kv
WorkingDirectory=/home/kv
ExecStart=/home/kv/kv-server
Restart=always

[Install]
WantedBy=multi-user.target
```

```bash
systemctl daemon-reload
systemctl enable --now kv
systemctl status kv
```

## 部署记录

- 时间：2026-11-02 ~ 11-03
- 实例：阿里云 ECS `ecs.e-c1m1.large`（2 vCPU / 2 GiB，Ubuntu 24.04，华南2 河源）
- 公网 IP：`47.120.35.74`（实例到期 2026-12-01）
- 访问：`http://47.120.35.74:8080/kv`
- 安全组：入方向放行 TCP 22、8080
- 验证：公网四条接口 + 404 通过；`kill -9` 后自动拉起；`reboot` 后服务自启、数据仍在

## 踩过的坑

- **多行粘贴写 unit 文件会乱**：`cat > … <<'EOF'` 一次粘 12 行时 `EOF` 提前出现、末行被当成命令；改用单行 `printf '第1行\n第2行\n' > 文件`
- **服务一启动就退出（`status=0/SUCCESS`）**：手动跑的那份 `kv-server` 还占着 8080，而 `main.go` 没处理 `ListenAndServe` 的错误，所以端口冲突也是静默退出。`ss -lntp | grep 8080` 找到进程杀掉即可
- **服务名不是程序名**：`systemctl status kv` 的 `kv` 来自文件名 `kv.service`，程序本体在 `ExecStart=/home/kv/kv-server`
- **`data.log` 落在哪由 `WorkingDirectory` 决定**：不设或设错，重启后“数据不见了”（其实只是写到别处）
