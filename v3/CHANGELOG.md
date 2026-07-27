# Changelog

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
