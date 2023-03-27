// ==UserScript==
// @name         VideoWithYou
// @namespace    https://github.com/Moonkey233/VideoWithYou
// @version      1.0.0
// @description  Different places, same video. A script that controls the synchronous play of video websites.
// @author       Moonkey_ & Iris
// @match        https://www.bilibili.com/*
// @match        https://*.youku.com/*
// @match        https://youku.com/*
// @match        https://v.qq.com/*
// @match        https://www.youtube.com/*
// @match        https://pan.quark.cn/*
// @match        https://www.aliyundrive.com/*
// @match        https://www.iqiyi.com/*
// @match        https://www.netflix.com/*
// @match        https://www.disneyplus.com/*
// @icon         icon
// @grant        GM_addStyle
// @grant        GM_setValue
// @grant        GM_getValue
// ==/UserScript==

//const url = "127.0.0.1";
const url = "Moonkey233.top";
const port = 1206

var reconnectID = 0;
var reconnectCnt = 0;
var intervalID = 0;
var serverCurrentDtime = 0;
var count = 0;

class MyPlayer {
	constructor() {

	}

	getType() {
		let href = window.location.href;
		if (href.indexOf('bilibili') != -1) {
			this.type = 'bilibili';
		} else if (href.indexOf('youku') != -1) {
			this.type = 'youku';
		} else if (href.indexOf('v.qq') != -1) {
			this.type = 'tx';
		} else if (href.indexOf('youtube') != -1) {
			this.type = 'yt';
		} else if (href.indexOf('quark') != -1) {
			this.type = 'quark';
		} else if (href.indexOf('aliyun') != -1) {
			this.type = 'aliyun';
		} else if (href.indexOf('iqiyi') != -1) {
			this.type = 'iqy';
		} else if (href.indexOf('netflix') != -1) {
			this.type = 'netflix';
		} else if (href.indexOf('disney') != -1) {
			this.type = 'disney';
		}
	}

	isUndefined() {
		this.getType();
		if (this.type == 'bilibili') {
			return (typeof player == 'undefined');
		} else if (this.type == 'youku') {
			return (typeof videoPlayer == 'undefined');
		} else if (this.type == 'tx') {
			return (typeof document.getElementsByTagName('video')[0] == 'undefined');
		} else if (this.type == 'yt' || this.type == 'quark' || this.type == 'aliyun' || this.tpye == 'iqy' || this.type == 'netflix' || this.type == 'disney') {
			return (typeof document.getElementsByTagName('video')[0] == 'undefined');
		}
	}

	isPaused() {
		this.getType();
		if (this.type == 'bilibili') {
			return player.isPaused();
		} else if (this.type == 'youku') {
			return (videoPlayer.getPlayerState().state == 'paused');
		} else if (this.type == 'tx') {
			return document.getElementsByTagName('video')[0].paused;
		} else if (this.type == 'yt' || this.type == 'quark' || this.type == 'aliyun' || this.tpye == 'iqy' || this.type == 'netflix' || this.type == 'disney') {
			return document.getElementsByTagName('video')[0].paused;
		}
	}

	isEnded() {
		this.getType();
		if (this.type == 'bilibili') {
			return player.isEnded();
		} else if (this.type == 'youku') {
			return videoPlayer.isEnd;
		} else if (this.type == 'tx') {
			return document.getElementsByTagName('video')[0].ended;
		} else if (this.type == 'yt' || this.type == 'quark' || this.type == 'aliyun' || this.tpye == 'iqy' || this.type == 'netflix' || this.type == 'disney') {
			return document.getElementsByTagName('video')[0].ended;
		}
	}

	getCurrentTime() {
		this.getType();
		if (this.type == 'bilibili') {
			return player.getCurrentTime();
		} else if (this.type == 'youku') {
			return videoPlayer.getCurrentTime();
		} else if (this.type == 'tx') {
			return document.getElementsByTagName('video')[0].currentTime;
		} else if (this.type == 'yt' || this.type == 'quark' || this.type == 'aliyun' || this.tpye == 'iqy' || this.type == 'netflix' || this.type == 'disney') {
			return document.getElementsByTagName('video')[0].currentTime;
		}
	}

	seek(time) {
		this.getType();
		if (this.type == 'bilibili') {
			player.seek(time);
		} else if (this.type == 'youku') {
			videoPlayer.seek(time);
		} else if (this.type == 'tx') {
			document.getElementsByTagName('video')[0].currentTime = time;
		} else if (this.type == 'yt' || this.type == 'quark' || this.type == 'aliyun' || this.tpye == 'iqy' || this.type == 'netflix' || this.type == 'disney') {
			document.getElementsByTagName('video')[0].currentTime = time;
		}
	}

	play() {
		this.getType();
		if (this.type == 'bilibili') {
			player.play();
		} else if (this.type == 'youku') {
			videoPlayer.play();
		} else if (this.type == 'tx') {
			document.getElementsByTagName('video')[0].play();
		} else if (this.type == 'yt' || this.type == 'quark' || this.type == 'aliyun' || this.tpye == 'iqy' || this.type == 'netflix' || this.type == 'disney') {
			document.getElementsByTagName('video')[0].play();
		}
	}

	pause() {
		this.getType();
		if (this.type == 'bilibili') {
			player.pause();
		} else if (this.type == 'youku') {
			videoPlayer.pause();
		} else if (this.type == 'tx') {
			document.getElementsByTagName('video')[0].pause();
		} else if (this.type == 'yt' || this.type == 'quark' || this.type == 'aliyun' || this.tpye == 'iqy' || this.type == 'netflix' || this.type == 'disney') {
			document.getElementsByTagName('video')[0].pause();
		}
	}

	getPlaybackRate() {
		this.getType();
		if (this.type == 'bilibili') {
			return player.getPlaybackRate();
		} else if (this.type == 'youku') {
			return videoPlayer.context.config.rate;
		} else if (this.type == 'tx') {
			return document.getElementsByClassName('txp_menuitem txp_current')[1].getAttribute('data-value');
		} else if (this.type == 'yt' || this.type == 'quark' || this.type == 'aliyun' || this.tpye == 'iqy' || this.type == 'netflix' || this.type == 'disney') {
			return document.getElementsByTagName('video')[0].playbackRate;
		}
	}

	setPlaybackRate(rate) {
		this.getType();
		if (this.type == 'bilibili') {
			player.setPlaybackRate(rate);
		} else if (this.type == 'youku') {
			if (rate == 2) {
				document.getElementsByClassName('kui-playrate-rate-item')[0].click();
			} else if (rate == 1.5) {
				document.getElementsByClassName('kui-playrate-rate-item')[1].click();
			} else if (rate == 1.25) {
				document.getElementsByClassName('kui-playrate-rate-item')[2].click();
			} else if (rate == 0.5) {
				document.getElementsByClassName('kui-playrate-rate-item')[4].click();
			} else {
				document.getElementsByClassName('kui-playrate-rate-item')[3].click();
			}
		} else if (this.type == 'tx') {
			let length = document.getElementsByClassName('txp_menuitem').length;
			for (let i = 0; i < length; i++) {
				if (document.getElementsByClassName('txp_menuitem')[i].getAttribute("data-value") == rate) {
					document.getElementsByClassName('txp_menuitem')[i].click();
				}
			}
		} else if (this.type == 'yt' || this.type == 'quark' || this.type == 'aliyun' || this.tpye == 'iqy' || this.type == 'disney') {
			document.getElementsByTagName('video')[0].playbackRate = rate;
		}
	}
}

const myPlayer = new MyPlayer();

var ws = {};
var panel = document.createElement('div');
var openBtn = document.createElement('div');
var minimizeButton = document.createElement('button');
var helpButton = document.createElement('button');
var createButton = document.createElement('button');
var joinButton = document.createElement('button');
var exitButton = document.createElement('button');
var copyButton = document.createElement('button');
var nameInput = document.createElement('input');
var uuidInput = document.createElement('input');
var title = document.createElement('span');
var roomInfo = document.createElement('span');
var roomId = document.createElement('span');
var lastTime = document.createElement('span');
var lastMsg = document.createElement('span');
var icon = document.createElement('span');

var global = {
	roomHostFlag: true,
	connectedFlag: false,
	sessionUuid: '',
	userName: '',
	roomName: '',
	saveTime: 0,
	hostNumber: 0,
};

(function () {
	'use strict';

	global = GM_getValue('global');
	if (typeof global == 'undefined') {
		global = {
			roomHostFlag: true,
			connectedFlag: false,
			sessionUuid: '',
			userName: '',
			roomName: '',
			saveTime: 0,
			hostNumber: 0,
		};
	}

	if ((global.roomHostFlag && new Date().getTime() - global.saveTime >= 5000) || (!global.roomHostFlag && global.saveTime != -1)) {
		console.log('new');
		initPanel();
		global.sessionUuid = generateUuid();
		global.hostNumber++;
		global.connectedFlag = false;
	} else {
		console.log('old');
		initPanel();
		global = GM_getValue('global');
		global.hostNumber++;
		GM_setValue('global', global);
		changePanel(1);
		connectServer(url, port);
		intervalID = setInterval(sendDataMsg, 500);
		panel.style.display = 'none';
		openBtn.style.display = 'block'
	}

})();

function connectServer(url = "127.0.0.1", port = 1206) {
	console.log('test000');
	ws = new WebSocket(`wss://${url}:${port}`, 'undefined', {
		// 指定自定义的CA证书
		ca: `-----BEGIN CERTIFICATE-----\nMIIDazCCAlOgAwIBAgIUTASMYRLAkZ4LmtRwxETkME5IxhUwDQYJKoZIhvcNAQELBQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMzAzMjYwMTQwMTRaFw0zMzAzMjMwMTQwMTRaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDXVETZNw1lHoHDTOGFsEEiANFSxZRPfvQUsk1ZmLdu5vb1wdkgR0/7r5J3Bu2Q5ilzZVWgkv7Esge2P5o9SVLjFf+zZp+g/GNukHRDih7qiKqJ/NP/9pcsmyJ4O6zr6Q4mlK2FUR+lRCOTsZvA5ZvO1y+ggrGNVfDgqSrDE13ND9XloCVNO0v7R3SFiWW3iUYa1LVaBxkfnhedpMRX+kovs0ASsaL7agRWIAyVo5tNHDBu8UxG+M2/WwseO8Aa0YbfL8ixfZ69uN7/nWF283jUMHFc39ZXUane3nm88pUHWO/P1grqrlD/8MZeduHcQ9gAJJ/iCFi7Xalm3jHWY6z7AgMBAAGjUzBRMB0GA1UdDgQWBBT5t5c19GpAtSdLKylSCX+Hmq3wcTAfBgNVHSMEGDAWgBT5t5c19GpAtSdLKylSCX+Hmq3wcTAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQBtNbwpKoem3px4r23WDvAC0cBH46JMR4+liwC9zrULW4pVmdXR2NHHmhpCxgHcZb83NTJPE03YsOIAC3qesoErwQMc1lNM3wRWATzEPasJYdaYJz9nEwN4kBIUeLDjw03IeLNTNv/x4F6rkM/hKRKqpJWPYBbEXZyTEgXmBlpd6LT0EC6eV2PCwhR0RC7iuIo+m3q+rSceQlTJxyUpYab2ULFmKqHyAtgS/UIJT77Fdj5admDf+OypFpVBaqTJOxKU6xzpwQLeBU9rVatgIZHKP4Iscr93QkrMqMvM8NW1r0TSvfcJnzdUH38DQ7RtYvoOpGOZ0LdtXpWJIudJQcte\n-----END CERTIFICATE-----`,
		rejectUnauthorized: false,
	});
	console.log('test111');
	if (reconnectID == 0) {
		wsListener();
	} else {
		if (++reconnectCnt >= 5) {
			alert('WebSocket服务器连接出错');
			global.connectedFlag = false;
			clearInterval(reconnectID);
			reconnectCnt = 0;
			reconnectID = 0;
		};
	}
	console.log('test222');
}

function wsListener() {
	// 监听WebSocket连接打开事件
	ws.addEventListener('open', () => {
		console.log('WebSocket连接已打开');

		if (reconnectID != 0) {
			clearInterval(reconnectID);
			reconnectCnt = 0;
			reconnectID = 0;
		}

		console.log(global);
		if (!global.connectedFlag) {
			if (global.roomHostFlag) {
				sendNonDataMsg('create');
			} else {
				sendNonDataMsg('join');
			}
		}
	});

	// 监听WebSocket接收到消息事件
	ws.addEventListener('message', event => {
		recvJson(event.data);
		// console.log('WebSocket收到消息:', event.data);
	});

	// 监听WebSocket关闭事件
	ws.addEventListener('close', event => {
		console.log('WebSocket连接已关闭:', event.code, event.reason);
	});

	// 监听WebSocket出错事件
	ws.addEventListener('error', error => {
		reconnectID = setInterval(connectServer(url, port), 1000);
		console.log('WebSocket出错:', error);
		// alert('WebSocket出错:', error);
	});
}

function changePanel(index = 0) {
	if (index == 0) {
		joinButton.style.display = 'block';
		createButton.style.display = 'block';
		nameInput.style.display = 'block';
		uuidInput.style.display = 'block';
		exitButton.style.display = 'none';
		roomId.style.display = 'none';
		roomInfo.style.display = 'none';
		copyButton.style.display = 'none';
		lastTime.style.display = 'none';
		lastMsg.style.display = 'none';
	} else {
		roomId.textContent = `房间ID: ${global.sessionUuid}`;
		roomInfo.textContent = `${global.roomName} 的房间, 人数👥: ${count}`;
		joinButton.style.display = 'none';
		createButton.style.display = 'none';
		nameInput.style.display = 'none';
		uuidInput.style.display = 'none';
		exitButton.style.display = 'block';
		roomInfo.style.display = 'block';
		roomId.style.display = 'block';
		copyButton.style.display = 'block';
		lastTime.style.display = 'block';
		lastMsg.style.display = 'block';
	}
}

function recvJson(data) {
	let object = JSON.parse(data);
	console.log(object);

	if (object['type'] == 'init') {
		console.log(global);
		global.connectedFlag = true;
		global.roomHostFlag = object['isRoomHost'];
		global.sessionUuid = object['uuid'];
		global.roomName = object['userName'];
		count = object['count'];
		serverCurrentDtime = object['timeStamp'] - new Date().getTime();

		changePanel(1);
		intervalID = setInterval(sendDataMsg, 500);

	} else if (object['type'] == 'error') {
		alert(object['msg']);

	} else if (object['type'] == 'exit') {
		changePanel(0);
		clearInterval(intervalID);
		ws.close();
	} else if (object['type'] == 'data') {
		serverCurrentDtime = object['timeStamp'] - new Date().getTime();
		lastTime.innerHTML = '最新同步时间: ' + new Date(object['timeStamp']).toLocaleString();
		count = object['count'];

		lastMsg.innerHTML = object['msg'];

		if (!global.roomHostFlag) {
			let clientUrl = window.location.href.split('?')[0];
			if (clientUrl[clientUrl.length - 1] == '/') {
				clientUrl = clientUrl.substring(0, clientUrl.length - 1);
			}
			if (object['url'] && clientUrl != object['url']) {
				clearInterval(intervalID);

				if (global.roomHostFlag) {
					global.saveTime = new Date().getTime();
				} else {
					global.saveTime = -1;
				}
				GM_setValue('global', global);
				window.open(object['url'], '_self');
			}
			if (!myPlayer.isUndefined() && !object['isEnded']) {
				if (myPlayer.getPlaybackRate() != object['playbackRate']) {
					myPlayer.setPlaybackRate(object['playbackRate']);
				}

				if (!object['isPaused'] && (Math.abs(object['serverTime'] - object['currentTime'] - (serverCurrentDtime + new Date().getTime() - myPlayer.getCurrentTime() * 1000)) >= 500)) {
					myPlayer.seek((serverCurrentDtime + new Date().getTime() - object['serverTime'] + object['currentTime']) / 1000 + 0.5);
					myPlayer.play();
				}

				if (myPlayer.isPaused() != object['isPaused']) {
					if (myPlayer.isPaused()) {
						myPlayer.play();
					} else {
						myPlayer.seek(object['currentTime'] / 1000);
						myPlayer.pause();
					}
				}
			}
		}

		roomId.textContent = `房间ID: ${global.sessionUuid}`;
		roomInfo.textContent = `${global.roomName} 的房间, 人数👥: ${count}`;

	}
}

function generateUuid() {
	const charList = ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'a', 'b', 'c', 'd', 'e', 'f'];
	let uuid = '';
	for (let i = 0; i < 16; i++) {
		uuid += charList[Math.floor(Math.random() * 16)];
	}
	let timeStamp = new Date().getTime();
	uuid += ':';
	uuid += Math.floor(timeStamp / 256000).toString(16).toLowerCase();
	return uuid;
}

function sendNonDataMsg(type) {
	let message = {};
	message['type'] = type;
	message['userName'] = global.userName;
	message['uuid'] = getUuid();
	message['isRoomHost'] = global.roomHostFlag;

	ws.send(JSON.stringify(message));
}

function sendDataMsg() {
	let message = {};
	message['type'] = 'data';
	message['uuid'] = getUuid();
	message['isRoomHost'] = global.roomHostFlag;
	message['userName'] = global.userName;

	let serverUrl = window.location.href.split('?')[0];
	if (serverUrl[serverUrl.length - 1] == '/') {
		serverUrl = serverUrl.substring(0, serverUrl.length - 1);
	}
	message['url'] = serverUrl;

	if (global.roomHostFlag && !myPlayer.isUndefined()) {
		message['serverTime'] = new Date().getTime() + serverCurrentDtime;
		message['currentTime'] = Math.round(myPlayer.getCurrentTime() * 1000);
		message['playbackRate'] = myPlayer.getPlaybackRate();
		message['isPaused'] = myPlayer.isPaused();
		message['isEnded'] = myPlayer.isEnded();
	}

	ws.send(JSON.stringify(message));

	let hostNumber = 0;
	let time = 0;
	hostNumber = GM_getValue('global');
	// console.log(typeof hostNumber);
	if (typeof hostNumber != 'undefined') {
		time = hostNumber.saveTime;
		hostNumber = hostNumber.hostNumber;
	} else {
		hostNumber = global.hostNumber;
	}

	if (global.hostNumber < hostNumber && global.roomHostFlag && new Date().getTime() - time < 5000) {
		sendNonDataMsg('exit');
		global.connectedFlag = false;
		global.roomHostFlag = false;
		global.sessionUuid = '';
		changePanel(0);
		clearInterval(intervalID);
	} else {
		global.saveTime = new Date().getTime();
		GM_setValue('global', global);
	}
}

function getUuid() {
	if (global.roomHostFlag && global.sessionUuid == '') {
		global.sessionUuid = generateUuid();
	}
	return global.sessionUuid;
}

function initPanel() {
	panel.setAttribute('id', 'indexPanel');
	document.body.appendChild(panel);

	icon.innerHTML = '❤';
	icon.style.position = 'absolute';
	// icon.style.top = '8px';
	icon.style.left = '8px';
	icon.style['font-size'] = '24px';
	icon.style.color = 'pink'

	openBtn.appendChild(icon);
	openBtn.setAttribute('id', 'openBtn');
	openBtn.style.display = 'none';
	openBtn.style.color = 'pink';
	document.body.appendChild(openBtn);

	GM_addStyle(`#indexPanel {
        position: fixed;
        bottom: 20px;
        right: 20px;
        width: 300px;
        height: 200px;
        background-color: #fff;
        border: 2px solid #ccc;
        border-radius: 16px 16px 16px 16px ;
        box-shadow: 5px 5px 15px rgba(0, 0, 0, 0.3);
        z-index: 1314520;
    }`);
	GM_addStyle(`#openBtn {
        position: fixed;
        bottom: 20px;
        right: 20px;
        width: 40px;
        height: 40px;
        background-color: #fff;
        border: 2px solid #ccc;
        border-radius: 16px 16px 16px 16px ;
        box-shadow: 5px 5px 15px rgba(0, 0, 0, 0.3);
        z-index: 1314520;
    }`);

	var isDragging = false;
	var startX, startY, currentX, currentY;
	panel.addEventListener('mousedown', function (e) {
		isDragging = true;
		startX = e.clientX;
		startY = e.clientY;
	});
	panel.addEventListener('mouseup', function (e) {
		isDragging = false;
	});
	panel.addEventListener('mousemove', function (e) {
		if (isDragging) {
			currentX = e.clientX - startX;
			currentY = e.clientY - startY;
			panel.style.left = panel.offsetLeft + currentX + 'px';
			panel.style.top = panel.offsetTop + currentY + 'px';
			startX = e.clientX;
			startY = e.clientY;
		}
	});

	openBtn.addEventListener('mouseup', function (e) {
		panel.style.display = 'block';
		openBtn.style.display = 'none';
	});

	minimizeButton.innerHTML = '-';
	minimizeButton.style.position = 'absolute';
	minimizeButton.style.top = '10px';
	minimizeButton.style.right = '10px';
	minimizeButton.style.width = '20px';
	minimizeButton.style.height = '20px';

	minimizeButton.addEventListener('click', function (e) {
		panel.style.display = 'none';
		openBtn.style.display = 'block'
	});

	panel.appendChild(minimizeButton);

	helpButton.innerHTML = '?';
	helpButton.style.position = 'absolute';
	helpButton.style.top = '10px';
	helpButton.style.right = '40px';
	helpButton.style.width = '20px';
	helpButton.style.height = '20px';

	helpButton.addEventListener('click', function (e) {
		window.open('https://github.com/Moonkey233/VideoWithYou', '_blank');
	});

	panel.appendChild(helpButton);

	createButton.innerHTML = '创建';
	createButton.style.position = 'absolute';
	createButton.style.top = '150px';
	createButton.style.left = '40px';
	createButton.style.width = '60px';
	createButton.style.height = '30px';

	createButton.addEventListener('click', function (e) {
		let nameValue = nameInput.value;
		if (nameValue != '') {
			global.userName = nameValue;
			global.roomHostFlag = true;

			connectServer(url, port);
		} else {
			alert('昵称不能为空!');
		}
	});

	panel.appendChild(createButton);

	joinButton.innerHTML = '加入';
	joinButton.style.position = 'absolute';
	joinButton.style.top = '150px';
	joinButton.style.right = '40px';
	joinButton.style.width = '60px';
	joinButton.style.height = '30px';

	joinButton.addEventListener('click', function (e) {
		let nameValue = nameInput.value;
		let uuidValue = uuidInput.value;
		if (nameValue != '' && uuidValue != '') {
			global.userName = nameValue;
			global.sessionUuid = uuidValue;
			global.roomHostFlag = false;

			connectServer(url, port);
		} else {
			alert('昵称或房间ID不能为空!');
		}
	});

	panel.appendChild(joinButton);

	exitButton.innerHTML = '退出';
	exitButton.style.position = 'absolute';
	exitButton.style.top = '150px';
	exitButton.style.right = '40px';
	exitButton.style.width = '60px';
	exitButton.style.height = '30px';
	exitButton.style.display = 'none';
	exitButton.style.color = 'red'

	exitButton.addEventListener('click', function (e) {
		sendNonDataMsg('exit');
		global.connectedFlag = false;
		global.roomHostFlag = false;
		global.sessionUuid = '';
		changePanel(0);
		clearInterval(intervalID);
	});

	panel.appendChild(exitButton);

	nameInput.type = 'text';
	nameInput.placeholder = '请输入昵称';
	nameInput.style.position = 'absolute';
	nameInput.style.top = '50px';
	nameInput.style.left = '30px';
	nameInput.style.width = '240px';
	nameInput.style.height = '30px';

	panel.appendChild(nameInput);

	uuidInput.type = 'text';
	uuidInput.placeholder = '请输入房间ID(仅加入房间)';
	uuidInput.style.position = 'absolute';
	uuidInput.style.top = '100px';
	uuidInput.style.left = '30px';
	uuidInput.style.width = '240px';
	uuidInput.style.height = '30px';

	panel.appendChild(uuidInput);

	title.innerHTML = '❤VideoWithYou';

	title.style.color = 'pink';
	title.style['font-size'] = '20px';


	title.style.position = 'absolute';
	title.style.top = '5px';
	title.style.left = '5px';

	panel.appendChild(title);

	roomInfo.style.position = 'absolute';
	roomInfo.style.top = '40px';
	roomInfo.style.left = '30px';
	roomInfo.style.display = 'none';
	roomInfo.style['font-size'] = '16px';
	roomInfo.style.color = 'blue'

	lastTime.style.position = 'absolute';
	lastTime.style.top = '120px';
	lastTime.style.left = '30px';
	lastTime.style.display = 'none';
	lastTime.style['font-size'] = '14px';
	lastTime.style.color = 'green'

	lastMsg.style.position = 'absolute';
	lastMsg.style.top = '60px';
	lastMsg.style.left = '30px';
	lastMsg.style.display = 'none';
	lastMsg.style['font-size'] = '16px';
	lastMsg.style.color = 'purple'

	roomId.style.position = 'absolute';
	roomId.style.top = '100px';
	roomId.style.left = '30px';
	roomId.style['font-size'] = '14px';
	roomId.style.display = 'none';

	panel.appendChild(roomInfo);
	panel.appendChild(lastTime);
	panel.appendChild(lastMsg);
	panel.appendChild(roomId);

	copyButton.innerHTML = '复制ID';
	copyButton.style.position = 'absolute';
	copyButton.style.top = '150px';
	copyButton.style.left = '40px';
	copyButton.style.width = '60px';
	copyButton.style.height = '30px';
	copyButton.style.display = 'none';
	copyButton.style.color = 'green'

	copyButton.addEventListener('click', function (e) {
		navigator.clipboard.writeText(global.sessionUuid);
		copyButton.innerHTML = '已复制';
		setTimeout(function () {
			copyButton.innerHTML = '复制ID';
		}, 2000);
	});

	panel.appendChild(copyButton);
}