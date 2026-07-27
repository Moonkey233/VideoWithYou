# 测试与验收

## 自动化测试

从 `v3` 目录运行：

```powershell
go test ./...
go test -race ./internal/roomserver ./internal/hostserver ./internal/localcert ./internal/sshtunnel ./local-client/internal/ws
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
- 本地 CA 稳定生成、域名验证和服务端证书续期判断
- profile 携带公开 CA 且不携带 CA 私钥
- IPv4 云回退使用 profile CA 完成 WSS 验证
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
| 本地证书首次生成 | 清洁测试环境首次启动服主 | 无外网和 TCP 80 依赖，WSS 可用 |
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
- `go test -race ./...`
- `go vet ./...`
- `npm audit --audit-level=high`：0 个已知漏洞
- `npm run typecheck`
- `npm run build`
- 发布 EXE 的 `--version` 与 `--help`
- 隔离目录中的服主初始化、朋友配置导入、自安装和内嵌扩展释放
- 本机真实 WebSocket 的本机线路与 IPv6 失败后 IPv4 云回退
- 360 像素宽度下的本机、云回退和双线路失败扩展 UI
- 真实 Windows 本机 `route=local` 和 ECDSA P-256 TLS
- `ipv6.moonkey.top:21314` 的 IPv6 TCP/TLS
- `moonkey.top:21314` 经 SSH 反向转发的 IPv4 TCP/TLS
- v2 `.vwyprofile` 隔离导入

发布文件：

```text
VideoWithYou-v3-windows-amd64.exe
SHA256 e40c5187a1d89c0c1bcb7311dcd5a14d5c7ed6051bae0663c9a5de8720142409
```

公网验证使用现有 Windows 防火墙、DDNS、云服务器 `sshd` 与安全组；未修改
这些基础设施配置。
