# VideoWithYou

多人同步观看浏览器视频或 MPC-BE 本地视频。

## 当前版本

**[VideoWithYou v3](v3/README.md)**：

- Windows 单一 `VideoWithYou.exe`
- 客户端和 Windows 房间服务端可在同一进程运行
- `ipv6.moonkey.top:21314` IPv6 直连优先
- 直连失败自动使用 `moonkey.top:21314` IPv4 云转发
- 两条路径进入同一个 Windows 房间服务
- profile 固定信任的本地 CA、SSH 反向隧道、断线恢复和线路 UI
- EXE 内嵌并自动释放 Edge/Chrome 扩展

安装请从 [v3 Windows 安装说明](v3/docs/INSTALL_WINDOWS.md) 开始。

## 历史版本

- `v2/`：独立 Linux/Windows 服务端 + 本地客户端
- `v1/`：早期 JavaScript/Tampermonkey 实现
