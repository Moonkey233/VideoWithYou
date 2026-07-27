# Changelog

## 3.0.2

- 修复完全没有 IPv6 的客户端连接 IPv4 云回退后 WebSocket 超时
- SSH 远端连接改为代理到本机真实 TCP `[::1]:21314`，不再直接承载 HTTP/WebSocket
- profile CA 继续验证 `ipv6.moonkey.top`，IPv4 云线路不依赖客户端 IPv6 或 AAAA 解析
- 增加真实 SSH Channel、ECDSA 本地 CA、WSS 协议升级与双向消息回归测试
- profile 仍为 v2，v3.0.1 客户端升级 EXE 后无需重新导入

## 3.0.1

- 公网 TLS 改为服主本地 ECDSA P-256 CA 签发，客户端通过 profile 内置 CA 验证
- 取消公网 TCP 80、ACME 和外部证书机构依赖
- 现有 v3.0.0 服主配置启动时自动迁移到本地 CA
- 本地服务端证书在到期前自动续期，服务端握手动态加载新证书
- 客户端 profile 升级为 v2，旧 profile 需要重新导出和导入
- 增加本地 CA、WSS 服务和云回退证书验证测试
- 本机服务探测不再为旧 ACME 首次签发固定等待 15 秒

## 3.0.0

- Windows 客户端内嵌房间服务端，生成统一 `VideoWithYou.exe`
- 公网业务端口改为 TCP 21314
- IPv6 直连优先，失败后强制 IPv4 云转发
- 云端使用 OpenSSH 反向端口转发，直连和云端共用同一房间状态
- 增加稳定客户端身份、session token 和 30 秒异常断线恢复
- 公网服务使用 ACME 自动签发和续期的 WSS
- 增加客户端 access token
- 增加 SSH Ed25519 密钥生成和云主机指纹 TOFU
- 扩展显示本机、IPv6 直连、IPv4 云转发、延迟和具体错误
- 增加滚动日志、损坏配置备份和非致命组件降级
- 浏览器扩展嵌入 EXE 并自动释放
- 增加当前用户自安装和开机启动
- 增加 Go 单元/集成测试、Race Detector、Vet、TypeScript 和 npm audit 验证
