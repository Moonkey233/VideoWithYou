# VideoWithYou
**Different places, same video. A script that controls the synchronous playback of video websites.**

一款适用于异地双人/多端多人共同观看视频，并自动同步视频链接/暂停状态/播放进度/播放倍速的脚本。
### 现已支持:
+ Bilibili(www.bilibili.com)
+ 优酷视频(*.youku.com)
+ 腾讯视频(v.qq.com)
+ 爱奇艺(www.iqiyi.com)
+ 夸克云盘(pan.quark.cn)
+ 阿里云盘(www.aliyundrive.com) (阿里云跳转问题, 成员需要手动打开视频界面再加入房间)
+ YouTube(www.youtube.com)
+ NetFlix(www.netflix.com) (未经测试)
+ Disney+(www.disneyplus.com) (未经测试)

### 正在研究:
+ 百度云盘(pan.baidu.com)
> 理论上支持HTML5播放器的视频网站均可使用, 百度网盘隐藏了视频元素, 暂未找到控制方法。如果您有想要添加的网站, 可向我留言反馈。

# 用途&原理
+ 传统的腾讯会议等以视频形式传输画面的软件，会受到腾讯会议服务器与双方客户端的带宽限制而导致视频码率不足，出现卡顿，掉帧或快速变化画面时的模糊。
+ 此脚本传递少量的播放信息，多个客户端与服务端交流通信，实现播放的同步，带宽开销依赖小，每个客户端都相当于自己在播放视频，观感能得到一定程度的提升。

### 优点:
+ 可双人/多人/多房间同时在线使用
+ 自动同步播放链接，暂停状态，播放进度，播放倍速
+ 自动生成不重复房间ID，可一键复制
+ 使用安全的WSS协议连接通信
+ 实时显示成员进退消息与总人数，成员页面实现自动重定向
+ 房主页面实现新窗口自动连接转移
+ 多网站支持

###### 欢迎测试使用，感谢反馈bug与意见！联系邮箱Moonkey233@foxmail.com，在GitHub issue页面留言亦可。

# 安装&依赖
+ 本脚本是一款基于油猴插件(TamperMonkey)的浏览器脚本，使用JavaScript语言编写。**经测试，在Windows11 OS的Edge与Chrome浏览器中，以及MAC OS的Chrome浏览器中均能使用。**理论上Safari、Firefox等任何一款支持油猴插件的浏览器均能使用(但未经测试)。

1. 油猴是一款适用于多浏览器平台的开源脚本管理库，它提供了部分API供开发者使用。您首先需要去到油猴插件的官网(https://www.tampermonkey.net) 或者各大浏览器的插件安装商店界面**安装好油猴插件**(Chrome等部分浏览器可能需要使用科学上网工具)，并为其设置**允许访问本地文件URL权限**。

2. 其次，**将VideoWithYou.js导入或复制进油猴插件并保存启用**。

3. 如果您使用开发者的服务器与域名通信，**您需要在浏览器中输入一次"https://Moonkey233.top:2333"**, 这将触发浏览器的不安全提示(这是由于服务端脚本的tls加密使用的CA证书与密钥均为OPENSSL自签名的，未得到CA机构的官方认可)，**需要手动点击一次确认继续访问**(可能需要打开更多/高级设置之类的才能看到信任网站或继续访问类似字样，且点击后不会有后续反应)。

4. 如果您使用自己的服务器与端口，您需要在VideoWithMe.js顶部**修改const变量url和port的值**为您的服务器域名/ip与端口，server.js中的监听端口也需要修改，并将server.js部署到您的云服务器，使用nodejs环境运行(node server.js)，**服务端需要安装nodejs并配置websocket依赖环境**，使得server.js能够正确引用导入ws模块(npm i ws)。且您需要**开放云服务器对应端口入方向的TCP连接防火墙**，您同样需要访问"https://yourURL:yourPort", 以使得浏览器信任自签名CA证书。

5. 最后，重新打开浏览器，即可在受支持的视频网站界面使用本脚本了。

# 使用须知&特性讲解
+ 操作逻辑很重要，建议仔细阅读，否则可能出现无法自动连接等问题。
### 对房主端:
以下页面均默认为受支持的视频网站页面，且均可可以互相穿插跳转。
+ 假设你现在在页面a，此时a页面的脚本保持连接，此时你打开了一个新的页面b（不管是从a点击新链接打开的，或者是直接单独开的窗口搜索打开的，或者是已有的非a页面的刷新），也就是说，在保持连接的a页面之外如果有新的页面打开，则脚本的连接就会自动转移到新的页面，旧页面a的连接会自动关闭。
+ 这是在保持连接的5秒内有效。
+ 如果你直接关闭了a窗口，也可以在**5秒内**打开或者刷新其他页面，又会自动连接至该页面，否则5秒后会断开连接。

### 对成员端:
+ 如果房主在某个主页选择视频，即房主连接的页面是没有视频播放的，成员端不受控制，如果房主选择了新的视频打开，成员的当前页面会重定向到房主选择的视频，控制与房主同步。
+ 但成员端只有在自动跳转视频的时候页面会自动重连，自动重连也可以跨网站跳转。但成员打开新页面时不会把连接从正在播放的页面转移过来。

# 贡献者
**由Moonkey_与Iris共同测试开发完成。感谢你的陪伴❤️With U**
