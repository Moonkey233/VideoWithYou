# Windows 安装与浏览器配置

## 一、服主电脑

服主电脑是 `ipv6.moonkey.top` 指向的 Windows 电脑。

### 1. 安装一个 EXE

下载 `VideoWithYou-v3-windows-amd64.exe`，在 PowerShell 中运行：

```powershell
.\VideoWithYou-v3-windows-amd64.exe --install --autostart
```

安装位置：

```text
%LOCALAPPDATA%\Programs\VideoWithYou\VideoWithYou.exe
```

该操作不需要管理员权限。`--autostart` 只写入当前用户的开机启动项。

当前个人构建未做商业 Authenticode 签名，Windows SmartScreen 可能首次提示。
先对照发布目录中的 SHA-256；确认一致后选择“更多信息 → 仍要运行”。这与公网
WSS 证书无关，不会要求朋友周期性重新安装。

### 2. 初始化混合服务端

```powershell
& "$env:LOCALAPPDATA\Programs\VideoWithYou\VideoWithYou.exe" --init-owner
```

它会自动完成：

- 启用 Windows 内嵌房间服务
- 生成客户端访问令牌
- 生成服主专属 ECDSA P-256 本地 CA 和 WSS 服务端证书
- 生成 SSH Ed25519 隧道密钥
- 生成客户端配置 `VideoWithYou-client.vwyprofile`
- 释放浏览器扩展

输出文件位于：

```text
%LOCALAPPDATA%\VideoWithYou
```

私钥、访问令牌和 `config.json` 不要发给朋友。只发送
`VideoWithYou-client.vwyprofile`。

### 3. 开放 Windows 防火墙

以管理员身份打开 PowerShell：

```powershell
New-NetFirewallRule `
  -DisplayName "VideoWithYou v3" `
  -Direction Inbound `
  -Action Allow `
  -Protocol TCP `
  -LocalPort 21314
```

如果光猫或路由器还有“IPv6 防火墙/入站规则”，也允许本机 TCP 21314。
IPv6 不需要传统 IPv4 端口转发。程序不再需要公网 TCP 80 或 443。

确认 DDNS：

```powershell
Resolve-DnsName ipv6.moonkey.top -Type AAAA
```

返回地址应与本机当前公网 IPv6 一致。

### 4. 配置云服务器

按照 [CLOUD_RELAY.md](CLOUD_RELAY.md) 配置。云端只需配置一次。

### 5. 正常启动

```powershell
& "$env:LOCALAPPDATA\Programs\VideoWithYou\VideoWithYou.exe"
```

启动成功后控制台应先显示：

```text
[服务端] IPv6 公网监听 address=[::]:21314
[隧道] SSH 反向转发已建立 remote=0.0.0.0:21314
[网络] 已连接 route=local
```

本地 CA 和服务端证书在初始化时立即生成，不访问外部证书机构。服务端证书
临近到期时由程序使用同一个本地 CA 自动续期。

## 二、朋友电脑

### 1. 安装

```powershell
.\VideoWithYou-v3-windows-amd64.exe --install --autostart
```

### 2. 导入客户端配置

将服主发送的 `VideoWithYou-client.vwyprofile` 保存到本地，然后：

```powershell
& "$env:LOCALAPPDATA\Programs\VideoWithYou\VideoWithYou.exe" `
  --import-profile ".\VideoWithYou-client.vwyprofile"
```

每台朋友电脑会生成自己的会话身份。导入不会启用服务端或 SSH 隧道。
profile 内包含服主 CA 的公开证书，用于验证 WSS；不会安装到 Windows 系统
证书库，也不包含 CA 私钥。

### 3. 启动

```powershell
& "$env:LOCALAPPDATA\Programs\VideoWithYou\VideoWithYou.exe"
```

线路选择完全自动：

1. 尝试 `ipv6.moonkey.top:21314` IPv6 直连。
2. 失败后连接 `moonkey.top:21314` IPv4 云转发。
3. 两边都失败时保持运行并持续重试。

## 三、加载 Edge/Chrome 扩展

EXE 会自动把扩展释放到：

```text
%LOCALAPPDATA%\VideoWithYou\extension
```

Edge：

1. 打开 `edge://extensions`
2. 开启“开发人员模式”
3. 点击“加载解压缩的扩展”
4. 选择 `%LOCALAPPDATA%\VideoWithYou\extension`

Chrome：

1. 打开 `chrome://extensions`
2. 开启“开发者模式”
3. 点击“加载已解压的扩展程序”
4. 选择 `%LOCALAPPDATA%\VideoWithYou\extension`

浏览器扩展只连接 `ws://127.0.0.1:23333/ext`，不需要配置公网地址或证书。

升级 EXE 后如扩展版本未刷新：

```powershell
VideoWithYou.exe --extract-extension
```

然后在扩展管理页点击该扩展的“重新加载”。

## 四、线路显示

扩展弹窗和控制台会显示：

- `本机服务端`：服主自己的客户端
- `IPv6 直连`：直接连接服主 Windows
- `IPv4 云转发`：IPv6 不可用，已自动回退
- `正在检测 IPv6`
- `正在切换到 IPv4 云转发`
- `两条线路均失败，等待重试`

切换线路时服务端保留会话 30 秒，正常情况下房间号和角色不会丢失。

## 五、从 v3.0.0 升级

先退出旧进程，再用 v3.0.1 发布文件执行：

```powershell
.\VideoWithYou-v3-windows-amd64.exe --install
```

程序会自动把现有服主配置从 ACME 迁移到本地 CA，并更新：

```text
%LOCALAPPDATA%\VideoWithYou\VideoWithYou-client.vwyprofile
```

服主不需要重新执行 `--init-owner`，云端 SSH 公钥和 access token 不变。由于
profile 格式从 v1 升级为 v2，朋友必须重新导入这份更新后的 profile：

```powershell
& "$env:LOCALAPPDATA\Programs\VideoWithYou\VideoWithYou.exe" `
  --import-profile ".\VideoWithYou-client.vwyprofile"
```
