# 故障排查

日志目录：

```text
%LOCALAPPDATA%\VideoWithYou\logs
```

主日志 `videowithyou.log` 达到约 5 MB 后自动轮转，最多保留 5 份。

## IPv6 直连失败但云端成功

这是正常回退。扩展会显示“IPv4 云转发”，同时保留 IPv6 失败原因。

检查：

```powershell
Resolve-DnsName ipv6.moonkey.top -Type AAAA
Test-NetConnection ipv6.moonkey.top -Port 21314
```

常见原因：

- 朋友所在网络没有 IPv6
- Windows 防火墙没有开放 TCP 21314
- 路由器/光猫 IPv6 入站防火墙拦截
- DDNS 尚未更新到当前公网 IPv6

## 两条线路都失败

```powershell
Test-NetConnection ipv6.moonkey.top -Port 21314
Test-NetConnection moonkey.top -Port 21314
```

如果直连失败且云端 21314 也失败，检查服主控制台中的“云回退隧道”状态、
云安全组和 `setup-cloud-relay.sh` 配置。

程序不会退出，会持续按“IPv6 → IPv4 云转发”的顺序重试。

## 证书签发或续期失败

检查：

```powershell
Resolve-DnsName ipv6.moonkey.top -Type AAAA
Test-NetConnection ipv6.moonkey.top -Port 80
```

注意：同一局域网内测试公网 IPv6 的结果可能受路由器策略影响，最好使用手机
热点或另一条外网验证。

必须满足：

- DDNS AAAA 指向服主当前 IPv6
- Windows EXE 正在运行
- Windows 和路由器允许 IPv6 TCP 80
- 系统时间正确

证书缓存：

```text
%LOCALAPPDATA%\VideoWithYou\certs
```

不要频繁删除缓存，否则可能触发 CA 申请频率限制。

## 云隧道报 access denied

确认：

- `%LOCALAPPDATA%\VideoWithYou\ssh\id_ed25519.pub` 已写入云端
- 云端用户名为 `videowithyou`
- SSH 地址为 `moonkey.top:22`
- `sshd -t` 通过
- SSH 服务已重载

## 客户端报 access denied

朋友导入了错误或过期的 `.vwyprofile`。服主重新导出：

```powershell
VideoWithYou.exe --export-profile .\VideoWithYou-client.vwyprofile
```

朋友重新执行 `--import-profile`。不需要重新加载浏览器扩展。

## 浏览器显示“本地客户端未连接”

确认 EXE 正在运行，然后检查：

```powershell
Test-NetConnection 127.0.0.1 -Port 23333
```

如果端口被其他程序占用，日志会显示 `ext ws listen failed`。关闭占用程序后重启
VideoWithYou。

## 房间在切换线路后消失

异常断线有 30 秒恢复时间。如果两条线路超过 30 秒都无法连接，房主会按正常
离线流程解散房间。检查日志中的：

```text
[服务端] 会话进入恢复宽限
[服务端] 会话已恢复
[服务端] 会话恢复超时
```
