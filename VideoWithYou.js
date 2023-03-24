// ==UserScript==
// @name         VideoWithYou
// @namespace    https://github.com/Moonkey233/VideoWithYou
// @version      0.9.0
// @description  Different places, same video. A script that controls the synchronous play of video websites.
// @author       Moonkey_ & Iris
// @match        https://www.bilibili.com/video/*
// @icon         icon
// @grant        GM_addStyle
// ==/UserScript==

//const url = "127.0.0.1";
const url = "Moonkey233.top";
//const url = "124.221.55.216";
const port = 1206
// const initTimeMs = 2000;
var ws;
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
var title = document.createElement('h3');
var roomInfo = document.createElement('textarea');
var roomId = document.createElement('textarea');
var roomHostFlag = true;
var connectedFlag = false;
var serverCurrentDtime = 0;
var sessionUuid = '';
var userName = '';
var roomName = '';
var count = 0;
var intervalID;

(function () {
	'use strict';

	initPanel();
	sessionUuid = generateUuid();

})();

function connectServer(url = "127.0.0.1", port = 8888) {
	const ws = new WebSocket(`ws://${url}:${port}`);
	wsListener(ws);
	return ws;
}

function wsListener(ws) {
	// 监听WebSocket连接打开事件
	ws.addEventListener('open', () => {
		console.log('WebSocket连接已打开');
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
		alert('WebSocket出错:', error);
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
	} else {
		roomId.textContent = `房间ID: ${sessionUuid}`;
		roomInfo.textContent = `${roomName} 的房间, 人数: ${count}`;
		joinButton.style.display = 'none';
		createButton.style.display = 'none';
		nameInput.style.display = 'none';
		uuidInput.style.display = 'none';
		exitButton.style.display = 'block';
		roomInfo.style.display = 'block';
		roomId.style.display = 'block';
		copyButton.style.display = 'block';
	}
}

function recvJson(data) {
	let object = JSON.parse(data);
	console.log(object);

	if (object['type'] == 'init') {
		connectedFlag = true;
		roomHostFlag = object['isRoomHost'];
		sessionUuid = object['uuid'];
		roomName = object['userName'];
		count = object['count'];
		serverCurrentDtime = object['timeStamp'] - new Date().getTime();

		changePanel(1);
		intervalID = setInterval(sendDataMsg, 1000);

	} else if (object['type'] == 'error') {
		alert(object['msg']);

	} else if (object['type'] == 'exit') {
		ws.close();
	} else if (object['type'] == 'data') {
		serverCurrentDtime = object['timeStamp'] - new Date().getTime();
		count = object['count'];
		if (!roomHostFlag) {
			console.log(Math.abs(object['serverTime'] - object['currentTime'] - (serverCurrentDtime + new Date().getTime() - player.getCurrentTime() * 1000)));
			if (player.getPlaybackRate() != object['playbackRate'] ||
				player.isPaused() != object['isPaused'] ||
				player.getManifest()['bvid'] != object['bvid'] ||
				Math.abs(object['serverTime'] - object['currentTime'] - (serverCurrentDtime + new Date().getTime() - player.getCurrentTime() * 1000)) >= 1000
			) {
				console.log("同步ing");
				if (player.isPaused() && !object['isPaused']) {
					player.play();
				}
				if (!player.isPaused() && object['isPaused']) {
					player.pause();
				}
				if (object['isPaused']) {
					player.seek(object['currentTime'] / 1000);
					player.pause();
				} else {
					player.seek((new Date().getTime() - object['serverTime'] + object['currentTime']) / 1000);
					player.play();
				}
				if (player.getPlaybackRate() != object['playbackRate']) {
					player.setPlaybackRate(object['playbackRate']);
				}
			}
		}

		roomId.textContent = `房间ID: ${sessionUuid}`;
		roomInfo.textContent = `${roomName} 的房间, 人数: ${count}`;

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
	message['userName'] = userName;
	message['uuid'] = getUuid();
	message['isRoomHost'] = roomHostFlag;

	ws.send(JSON.stringify(message));
}

function sendDataMsg() {


	let message = {};
	message['type'] = 'data';
	message['uuid'] = getUuid();
	message['isRoomHost'] = roomHostFlag;

	if (roomHostFlag) {
		message['serverTime'] = new Date().getTime() + serverCurrentDtime;
		message['currentTime'] = Math.round(player.getCurrentTime() * 1000);
		message['playbackRate'] = player.getPlaybackRate();
		message['isPaused'] = player.isPaused();
		message['isEnded'] = player.isEnded();
		message['bvid'] = player.getManifest()['bvid'];
	}

	ws.send(JSON.stringify(message));
}

function getUuid() {
	if (roomHostFlag && sessionUuid == '') {
		sessionUuid = generateUuid();
	}
	return sessionUuid;
}

function initPanel() {
	// 创建浮动面板窗口
	panel.setAttribute('id', 'indexPanel');
	document.body.appendChild(panel);

	// 创建浮动面板窗口
	openBtn.setAttribute('id', 'openBtn');
	openBtn.innerHTML = 'Open';
	document.body.appendChild(openBtn);
	openBtn.style.display = 'none';

	// 样式
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
        z-index: 99999;
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
        z-index: 99999;
        background-image: url(img.src) no-repeat center center;
    }`);

	// 拖动窗口
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
			userName = nameValue;
			roomHostFlag = true;

			ws = connectServer(url, port);

			setTimeout(function () {
				sendNonDataMsg('create');
			}, 200);

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
			userName = nameValue;
			sessionUuid = uuidValue;
			roomHostFlag = false;

			ws = connectServer(url, port);

			setTimeout(function () {
				sendNonDataMsg('join');
			}, 200);

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

	exitButton.addEventListener('click', function (e) {
		sendNonDataMsg('exit');
		connectedFlag = false;
		roomHostFlag = false;
		sessionUuid = '';
		changePanel(0);
		clearInterval(intervalID);
	});

	panel.appendChild(exitButton);

	// 创建姓名输入框
	nameInput.type = 'text';
	nameInput.placeholder = '请输入昵称';
	nameInput.style.position = 'absolute';
	nameInput.style.top = '50px';
	nameInput.style.left = '30px';
	nameInput.style.width = '240px';
	nameInput.style.height = '30px';

	// 将姓名输入框添加到面板中
	panel.appendChild(nameInput);

	uuidInput.type = 'text';
	uuidInput.placeholder = '请输入房间ID(仅加入房间)';
	uuidInput.style.position = 'absolute';
	uuidInput.style.top = '100px';
	uuidInput.style.left = '30px';
	uuidInput.style.width = '240px';
	uuidInput.style.height = '30px';

	// 将姓名输入框添加到面板中
	panel.appendChild(uuidInput);

	// 创建标题元素

	title.textContent = 'VideoWithYou';
	uuidInput.style.position = 'absolute';
	title.style.top = '10px';
	title.style.left = '10px';

	// 将标题添加到面板中
	panel.insertBefore(title, panel.firstChild);

	// 创建姓名输入框
	roomInfo.style.position = 'absolute';
	roomInfo.style.top = '50px';
	roomInfo.style.left = '30px';
	roomInfo.style.width = '240px';
	roomInfo.style.height = '30px';
	roomInfo.style.display = 'none';

	// 创建姓名输入框
	roomId.style.position = 'absolute';
	roomId.style.top = '100px';
	roomId.style.left = '30px';
	roomId.style.width = '240px';
	roomId.style.height = '30px';
	roomId.style.display = 'none';

	// 将姓名输入框添加到面板中
	panel.appendChild(roomInfo);
	panel.appendChild(roomId);

	copyButton.innerHTML = '复制ID';
	copyButton.style.position = 'absolute';
	copyButton.style.top = '150px';
	copyButton.style.left = '40px';
	copyButton.style.width = '60px';
	copyButton.style.height = '30px';
	copyButton.style.display = 'none';

	copyButton.addEventListener('click', function (e) {
		navigator.clipboard.writeText(sessionUuid);
		copyButton.innerHTML = '已复制';
		setTimeout(function () {
			copyButton.innerHTML = '复制ID';
		}, 3000);
	});

	panel.appendChild(copyButton);
}