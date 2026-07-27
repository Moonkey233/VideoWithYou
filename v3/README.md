# VideoWithYou v3

VideoWithYou v3 是一个多人同步看视频工具。Windows 端只有一个主程序
`VideoWithYou.exe`，可同时承担以下角色：

- 浏览器扩展的本地桥接
- 视频同步客户端
- Windows 房间服务端（仅服主配置启用）
- IPv6 公网直连入口
- 到 Linux 云服务器的 SSH IPv4 反向转发
- 自动本地 TLS 证书、续期、日志和故障回退

## 连接架构

```text
普通客户端
  ├─ IPv6 可用 ───────────> ipv6.moonkey.top:21314 ─┐
  └─ IPv6 失败 -> moonkey.top:21314 -> SSH 隧道 ───┤
                                                     v
                                        同一个 Windows 房间服务
```

IPv6 直连和 IPv4 云转发最终进入同一个房间服务，因此不会出现两边房间号、
成员或播放状态不一致的问题。云服务器只转发直连失败用户的加密流量。

## 端口

| 位置 | 端口 | 用途 |
|---|---:|---|
| Windows 本机 | TCP 23333 | 浏览器扩展连接本地 EXE，仅监听 127.0.0.1 |
| Windows 公网 IPv6 | TCP 21314 | WSS 视频同步服务 |
| Linux 云公网 IPv4 | TCP 21314 | IPv4 回退入口，经 SSH 转发回 Windows |
| Linux 云服务器 | TCP 22 | Windows 建立 SSH 反向隧道 |

v2 使用的 9012 不受 v3 影响。

## 快速入口

- [Windows 安装、服主配置和朋友安装](docs/INSTALL_WINDOWS.md)
- [Linux 云端 IPv4 转发配置](docs/CLOUD_RELAY.md)
- [故障排查](docs/TROUBLESHOOTING.md)
- [测试和验收记录](docs/TESTING.md)
- [开发与构建](scripts/dev_readme.md)

## 常用命令

```powershell
# 安装到当前用户目录，可选开机启动
.\VideoWithYou-v3-windows-amd64.exe --install --autostart

# 将本机初始化为 Windows 服务端 + 客户端
VideoWithYou.exe --init-owner

# 普通客户端导入服主发来的连接配置
VideoWithYou.exe --import-profile .\VideoWithYou-client.vwyprofile

# 重新释放浏览器扩展并显示目录
VideoWithYou.exe --extract-extension

# 导出新的普通客户端配置
VideoWithYou.exe --export-profile .\VideoWithYou-client.vwyprofile
```

配置、证书、SSH 密钥、扩展和日志默认位于：

```text
%LOCALAPPDATA%\VideoWithYou
```

程序不会因 DNS、IPv6、云端、证书、SSH 隧道或监听端口等可恢复错误闪退。
控制台、日志和扩展 UI 会分别显示 IPv6 失败原因、云端失败原因及当前线路。

## 证书

公网线路默认使用 WSS。服主初始化时生成专属 ECDSA P-256 本地 CA，由它自动签发服务端
证书；朋友 profile 内只包含公开 CA 证书，EXE 用它验证服主身份。朋友无需
安装系统证书，也不需要按证书周期重新导入配置。服主只需长期保持：

- `ipv6.moonkey.top` 的 AAAA 指向当前 Windows 公网 IPv6
- IPv6 TCP 21314 能从公网访问

本地 CA 私钥只保存在服主 `%LOCALAPPDATA%\VideoWithYou\certs`。公网不再
需要 TCP 80、443 或外部 ACME 证书机构。

## 源码结构

- `internal/roomserver`：共享房间服务与断线恢复
- `internal/hostserver`：Windows IPv6/WSS
- `internal/localcert`：服主本地 CA、证书签发与续期
- `internal/sshtunnel`：内嵌 SSH 反向转发
- `local-client`：同步客户端、本地浏览器桥、Browser/MPC-BE 适配
- `extension`：Chrome/Edge Manifest V3 扩展
- `server/cmd/server`：仅供开发和回滚的独立兼容服务端
- `scripts`：构建、云端配置和网络验证

## 构建

```powershell
cd v3
.\scripts\build.ps1
```

脚本会执行扩展类型检查、扩展构建、Go 测试、Go Vet、Windows EXE 构建、
Linux 兼容服务端构建，并生成 SHA-256：

```text
v3\release\VideoWithYou-v3-windows-amd64.exe
v3\release\VideoWithYou-v3-windows-amd64.sha256
```
