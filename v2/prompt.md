你是资深全栈/系统工程师。请从零生成一个可运行的 monorepo：VideoWithYou v2，在v2目录中新实现，不要修改其他已有文件。
目标：多人同步看视频。Edge/Chrome 扩展只与本地客户端通信；本地客户端负责与服务器通信、同步核心逻辑、以及接入 Browser/PotPlayer 两种播放端。协议采用 Protobuf。

【技术栈】
- Server：Go 1.22+，WebSocket（二进制帧承载 protobuf）
- Local Client：Go 1.22+，常驻进程（控制台先行即可，可选托盘），实现：
  1) 与 Server 的 WS 连接 + 重连 + ping/pong keepalive
  2) NTP 风格时钟同步（4 timestamps，计算 offset、delay）
  3) 同步核心（host 上报 state，follower 对齐）
  4) Native Messaging Host（stdin/stdout JSON）与浏览器扩展通信
  5) Endpoint = Browser 或 PotPlayer 二选一（配置可切换）
  6) Browser Endpoint 下有 follow_url 开关（是否自动跳转到 host 的 URL）
- Extension：Manifest V3 + TypeScript（建议 Vite），包含：
  - service worker：连接 Native Messaging；转发 UI/状态消息
  - content script：站点适配器（Bilibili + Quark + Generic HTML5兜底），读取/设置 video URL/currentTime/paused/ended/playbackRate
  - UI：popup + 可选页面 overlay（先 popup 也行）

【不需要实现】
- 安全合规（证书、鉴权、权限最小化等）
- 多协议版本
- PotPlayer 自动打开/自动换片（PotPlayer 只做时间/暂停/倍速同步；可选实现“设置播放位置 seek”）

============================================================
一、功能需求
============================================================

1) 房间基本功能
- create room：创建房间，成为 host
- join room：加入房间，成为 follower
- leave room：退出房间
- server 维护 room -> members -> host_id -> latest_state

2) 同步内容
- 必须同步：paused/play、position_ms、rate
- 允许：offset_ms（follower端偏移，可手工设定本地偏移）
- Browser follower 可选 follow_url：若开启且 Endpoint=Browser，则在 host media.url 变化时自动跳转

3) Endpoint 选择
- 本地客户端配置项：
  - endpoint: "browser" | "potplayer"
  - follow_url: true/false（仅 endpoint=browser 生效）
- endpoint=potplayer：
  - 仅对齐 pause/play、rate、position_ms（seek 可选）
  - 不进行任何 URL 跳转
- endpoint=browser：
  - content script 对齐 HTML5 video 状态
  - follow_url=true 时，收到 host 的 media.url 与当前不同则 navigate

4) 可配置参数（本地客户端 config）
- tick_ms：host 上报频率（例如 500ms，默认可调）
- hard_seek_threshold_ms：漂移超过该阈值直接 seek（默认 1000ms）
- deadzone_ms：漂移小于该阈值不动作（默认 200ms）
- soft_rate_enabled：是否启用倍速微调（默认 true）
- soft_rate_threshold_ms：进入微调区间的阈值（默认 600ms）
- soft_rate_adjust：微调倍速幅度（默认 0.02）
- offset_ms：本地额外偏移（可选）

============================================================
二、同步算法（必须实现）
============================================================

A) NTP 风格时钟同步（client<->server）
实现 TimeSyncReq/Resp：
- 客户端发送 req 时记录 t1_local_ms
- server 收到记录 t2_server_ms，并在发回 resp 前记录 t3_server_ms
- 客户端收到 resp 记录 t4_local_ms
计算：
- delay = (t4 - t1) - (t3 - t2)
- offset = ((t2 - t1) + (t3 - t4)) / 2
其中 offset 表示 server_time - local_time
策略：
- 连接建立后连续探测 N=5 次，取 delay 最小的一次的 offset 作为 current_offset
- 每隔 600s(可配置) 低频刷新一次

B) follower 目标进度预测
收到 HostState（含 sample_server_time_ms）时：
- now_server_ms = now_local_ms + offset
- 若 paused：target = position_ms + offset_ms
- 若 playing：target = position_ms + offset_ms + (now_server_ms - sample_server_time_ms) * rate

C) 漂移控制（本地客户端执行）
- drift = target - local_position
- |drift| < deadzone_ms：不动作
- |drift| >= hard_seek_threshold_ms：seek 到 target
- 否则若 soft_rate_enabled：
  - 临时将 rate 调为 host_rate ± soft_rate_adjust（drift>0 用更快，drift<0 用更慢）
  - 持续最多 soft_rate_max_ms（例如 3000ms），或 drift 进入 deadzone 后恢复 host_rate

注意：Browser/PotPlayer 的“读状态/写状态”由各自 adapter 实现；同步决策统一在 SyncCore。

============================================================
三、Protobuf 协议
============================================================

在 /proto/videowithyou.proto 定义：
- Envelope { oneof payload }
- ClientHello/ServerHello
- CreateRoomReq/Resp, JoinRoomReq/Resp, LeaveRoomReq
- RoomSnapshot（members + latest_state）
- HostState（host->server）
- BroadcastState（server->followers）
- TimeSyncReq/TimeSyncResp
字段要求：
- 全部时间单位使用 *_ms（int64 / uint64）
- HostState 必须包含：
  - room_id, host_id, seq, media(url,title,site,attrs), position_ms, rate, paused, sample_server_time_ms, offset_ms
- 任何广播都带 server_time_ms（由 server 填充）

============================================================
四、Server 详细需求（Go）
============================================================

1) WebSocket 入口：
- 监听 :2333（可配置）
- 路径：/ws
- 客户端连接后先发 ClientHello，server 回 ServerHello（含 server_time_ms）
- server 支持 binary frames（protobuf Envelope）
- server 主动 ping（例如每 15s），并处理 pong，断线清理成员

2) 房间与状态
- room_code：短码（随机生成），映射到 room_id
- room 内：
  - host_id
  - members map
  - latest HostState
- 广播策略：
  - 收到 host 的 HostState：更新 latest，并立即 BroadcastState 给 room 内所有 follower
  - join/leave 也广播 RoomSnapshot 或 BroadcastState（包含 members）

============================================================
五、Local Client 详细需求（Go）
============================================================

1) 结构：
- cmd/local-client/main.go：启动，加载 config，启动三条 goroutine：
  a) WS client -> server
  b) Native Messaging host (stdin/stdout JSON) -> extension
  c) SyncCore ticker（基于 tick_hz），推动 host 上报或 follower 对齐

2) 与 extension 的 Native Messaging（JSON）
定义 JSON 消息类型（不用 protobuf）：
- Ext->Client:
  - "ext_hello": {tabId,url,site,version}
  - "player_state": {position_ms,duration_ms,paused,rate,media:{url,title,site,attrs}}
  - "ui_action": {"create_room"|"join_room"|"leave_room"|"set_endpoint"|"set_follow_url"|"set_config", payload...}
- Client->Ext:
  - "apply_state": {position_ms,paused,rate}
  - "navigate": {url}
  - "ui_state": {room_code, role(host/follower), members_count, endpoint, follow_url, last_error}

3) Endpoint adapters（本地客户端内部）
- BrowserAdapter：
  - 状态来源：extension 上报的 player_state
  - 执行动作：向 extension 发 apply_state / navigate
- PotPlayerAdapter（v1：尽可能可用，先不追求完美）
  - 可选实现 A（优先）：通过调用 PotPlayer.exe 命令行 /seek=hh:mm:ss.ms 或 /seek=seconds 来定位（seek）
  - play/pause、rate：先用“发送快捷键/模拟按键”到 PotPlayer 窗口（实现一套可配置热键映射）
  - get current position：若无法可靠读取，可退化为“仅在大漂移时 seek”，本地位置估计 = 上次对齐时刻 + (now - last_sync)*rate
  - 所有 PotPlayer 行为都写入日志，便于后续替换为更稳的 Win32/COM/UIAutomation 控制实现

4) 角色与状态
- role: host | follower
- host：周期性发送 HostState（包含 sample_server_time_ms=当前 server_time）
- follower：接收 BroadcastState，计算 target 并对齐到当前 endpoint

============================================================
六、Extension 详细需求（TS, MV3）
============================================================

1) 文件：
- extension/manifest.json（MV3）
- extension/src/sw.ts：service worker
- extension/src/content.ts：content script
- extension/src/sites/bilibili.ts, quark.ts, generic.ts：适配器
- extension/src/ui/popup.tsx（可选用 react），或纯 html/js

2) Content script 站点适配
- IVideoAdapter：
  - detect(): boolean / score
  - attach(): 绑定 video 元素（优先可见、面积最大）
  - readState(): position_ms, paused, rate, media(url,title,site,attrs)
  - applyState(target): set currentTime / play() / pause() / playbackRate
  - observe(): 监听 timeupdate/play/pause/ratechange + SPA 路由变化（MutationObserver + history patch）
- B站：
  - media.attrs 至少包含 bv（从 URL 提取）
- 夸克：
  - 先用 generic HTML5 方式获取 video；site="quark"

3) SW 与 Native Messaging
- SW 使用 chrome.runtime.connectNative(hostName)
- SW 收到 content script 状态，转发给本地客户端
- SW 收到本地客户端 apply_state/navigate，转发给对应 tab 的 content script 执行

============================================================
七、工程输出要求（必须满足）
============================================================

1) monorepo 结构
- /proto/videowithyou.proto
- /server (Go)
- /local-client (Go)
- /extension (TS, MV3)
- /scripts
  - build_server.sh / build_client.sh
  - install_native_host_windows.ps1（把 native host manifest 写入正确位置）
  - dev_readme.md（如何跑通）

2) 日志与可观测性
- local-client 输出关键日志：ws连接、ntp offset/delay、drift、采取的动作（seek/rate adjust）
- server 输出：房间创建、join/leave、广播次数

============================================================
八、实现细节建议（你需要直接照做）
============================================================

- WS：使用websocket（server+client）
- protobuf：google.golang.org/protobuf + protoc 生成
- 直接让 extension 只处理 JSON（因为 extension->client 走 Native Messaging JSON）

最后：生成全部源码（可直接编译运行），并确保 README 的步骤能在 Windows 上跑通。
