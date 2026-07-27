# VideoWithYou v3 开发说明

## 环境

- Go 1.22+
- Node.js 20+
- npm
- 可选：`rsrc`，用于更新 Windows EXE 图标
- 可选：Python + Pillow，用于从 `VideoWithYou.png` 生成图标

## 完整构建

```powershell
cd v3
.\scripts\build.ps1
```

构建顺序：

1. `npm ci`（没有 node_modules 时）
2. 扩展 TypeScript 类型检查
3. Vite 生产构建
4. 将扩展复制到 Go embed 目录
5. Go 测试和 Vet
6. Windows 单 EXE 构建
7. 独立兼容服务端 Windows/Linux 构建
8. 生成 release EXE 和 SHA-256

## 单独运行开发服务端

独立服务端只用于协议开发和回滚：

```powershell
go run ./server/cmd/server -addr :21314
```

它默认是明文 WS。正式公网部署使用统一 Windows EXE 的 WSS/ACME 服务。

## 本地无 TLS 调试

复制 `local-client/config.json` 到临时目录，将：

```json
{
  "connection": {
    "direct_url": "ws://[::1]:21314/ws",
    "cloud_dial_address": ""
  },
  "server": {
    "enabled": true,
    "listen_address": "[::1]:21314",
    "tls": {
      "mode": "disabled"
    }
  },
  "relay": {
    "enabled": false
  }
}
```

然后：

```powershell
go run ./local-client/cmd/local-client --config .\tmp-config.json
```

不要把 `tls.mode=disabled` 用于公网。

## Protobuf

修改 `proto/videowithyou.proto` 后：

```powershell
protoc `
  --proto_path=proto `
  --plugin=protoc-gen-go="$env:USERPROFILE\go\bin\protoc-gen-go.exe" `
  --go_out=proto/gen `
  --go_opt=paths=source_relative `
  videowithyou.proto
```
