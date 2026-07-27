# Linux 云服务器 IPv4 回退配置

云服务器不再运行第二套房间服务，只通过 OpenSSH 把公网 IPv4 TCP 21314
转发到服主正在运行的 Windows EXE。

## 前提

- `moonkey.top` 的 A 记录指向云服务器 IPv4
- 云服务器安装并运行 OpenSSH Server
- 云安全组允许 TCP 22 和 TCP 21314
- Windows 已运行一次 `VideoWithYou.exe --init-owner`

## 自动配置

Windows 生成的 SSH 公钥位于：

```text
%LOCALAPPDATA%\VideoWithYou\ssh\id_ed25519.pub
```

把以下两个文件上传到 Linux：

- `v3/scripts/setup-cloud-relay.sh`
- `id_ed25519.pub`

在 Linux 执行：

```bash
sudo bash setup-cloud-relay.sh ./id_ed25519.pub
```

脚本会：

- 创建只能做转发的 `videowithyou` 用户
- 禁止密码认证、TTY、Agent 和 X11 转发
- 只允许远程 TCP 转发
- 只允许监听 `0.0.0.0:21314`
- 验证 sshd 配置后平滑重载
- 如果 UFW 已启用，放行 TCP 21314

还需要在阿里云/其他云厂商安全组中手工放行：

```text
入方向 TCP 21314，来源 0.0.0.0/0
```

不需要在云端开放 TCP 80。证书验证发生在 Windows 的 IPv6 TCP 80。

## 验证

先启动 Windows `VideoWithYou.exe`，然后在 Linux 查看：

```bash
ss -ltnp | grep ':21314'
```

远程转发只在 Windows 隧道在线时存在。Windows 控制台应显示：

```text
[隧道] SSH 反向转发已建立 remote=0.0.0.0:21314
```

从一台只有 IPv4 或临时禁用 IPv6 的电脑运行：

```powershell
Test-NetConnection moonkey.top -Port 21314
```

应显示 `TcpTestSucceeded : True`，扩展显示“IPv4 云转发”。

## 与 v2 并行迁移

v3 使用 21314，v2 的 9012 可以继续运行。建议顺序：

1. 保持 v2 的 9012 在线。
2. 配置并验证 v3 的 21314。
3. 在测试客户端导入 v3 profile。
4. 验证 IPv6、IPv4 云转发和断线恢复。
5. 全部切换后再决定是否停止 v2。

## SSH 主机密钥

Windows 第一次连接云 SSH 时采用 TOFU：

- 第一次保存云服务器 SSH 主机指纹
- 后续指纹改变时拒绝连接并在窗口报警

指纹位于：

```text
%LOCALAPPDATA%\VideoWithYou\ssh\host_key.pin
```

云服务器重装并确认新指纹无误后，删除该文件即可重新信任。
