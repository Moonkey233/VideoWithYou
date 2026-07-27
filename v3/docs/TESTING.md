# 测试与验收

## 自动化测试

从 `v3` 目录运行：

```powershell
go test ./...
go test -race ./internal/roomserver ./internal/hostserver ./internal/sshtunnel ./local-client/internal/ws
go vet ./...

cd extension
npm ci
npm audit
npm run typecheck
npm run build
```

自动化覆盖：

- 房间创建、加入与成员快照
- access token 拒绝
- 主机异常断线后使用同一 session token 恢复原房间
- IPv6 DNS/连接失败后转入强制 IPv4 云线路
- 云线路连接成功后仍保留 IPv6 失败原因
- SSH Ed25519 密钥生成与稳定加载
- SSH 主机密钥首次信任和变化拒绝
- 同一房间 HTTP 服务接受额外转发 Listener
- 配置生成、21314 默认端口、服主/朋友 profile 隔离
- 扩展 TypeScript 严格类型检查和生产构建

## 上线前人工验收矩阵

| 场景 | 操作 | 预期 |
|---|---|---|
| IPv6 直连 | IPv6 网络正常启动朋友端 | UI 显示“IPv6 直连” |
| IPv4 回退 | 在朋友端临时禁用 IPv6 | 数秒内显示“IPv4 云转发” |
| IPv6 端口阻断 | 临时阻断服主 IPv6 21314 | 自动回退云端，同一房间可加入 |
| 线路中断恢复 | 房间内中断当前线路后恢复另一条 | 30 秒内房间号和角色不变 |
| 云端不可用 | 暂停 SSH 隧道但保留 IPv6 | IPv6 用户继续正常使用 |
| 双线路不可用 | 同时阻断直连和云端 | EXE 不退出，UI 显示两个原因并重试 |
| 证书首次签发 | 清洁测试环境首次启动服主 | TCP 80 验证后 WSS 可用 |
| 浏览器桥异常 | 占用本机 23333 | EXE 保持运行并写明端口错误 |
| 配置损坏 | 使用无效 JSON 启动 | 备份 `.bad-时间`，使用安全默认配置 |

## 网络自检

```powershell
.\scripts\verify-network.ps1
```

该脚本只做 DNS 和 TCP 可达性检查，不修改系统配置。

## 2026-07-27 本机构建记录

以下检查已通过：

- `go test ./...`
- `go test -race ./internal/roomserver ./internal/hostserver ./internal/sshtunnel ./local-client/internal/ws ./local-client/internal/embedded`
- `go vet ./...`
- `npm audit --audit-level=high`：0 个已知漏洞
- `npm run typecheck`
- `npm run build`
- 发布 EXE 的 `--version` 与 `--help`
- 隔离目录中的服主初始化、朋友配置导入、自安装和内嵌扩展释放
- 本机真实 WebSocket 的本机线路与 IPv6 失败后 IPv4 云回退
- 360 像素宽度下的本机、云回退和双线路失败扩展 UI

发布文件：

```text
VideoWithYou-v3-windows-amd64.exe
SHA256 3898979344d1badb1582fb9106a81246c27cd8a52da7ce216f62bc2febf5ce91
```

尚未在这次本地构建中改动真实 Windows 防火墙、云服务器 `sshd` 或公网 DNS，
因此首次 ACME 签发和公网端到端连通性需按安装文档在正式环境完成最后验收。
