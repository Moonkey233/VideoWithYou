const fs = require('fs');
const WebSocket = require('ws');
const https = require('https');
const port = 2333
const options = {
	key: fs.readFileSync('cert/plutocharon.love.key'),
	cert: fs.readFileSync('cert/plutocharon.love.crt'),
	rejectUnauthorized: false
};
const server = https.createServer(options);
const wss = new WebSocket.Server({ server });
var socketPool = {};

server.listen(port, () => {
	console.log(`已开始监听: 0.0.0.0: ${port}`);
});

setInterval(removeRoom, 1000);
setInterval(getRealCnt, 2000);

wss.on('connection', function connection(ws) {
	console.log('Client connected');

	ws.on('message', function incoming(message) {
		console.log('received: %s', message);
		var object = JSON.parse(message)

		if (object['type'] == 'create') {
			socketPool[object['uuid']] = { member: {} };

			let msg = {};
			msg['type'] = 'init';
			msg['userName'] = object['userName'];
			msg['timeStamp'] = new Date().getTime();
			msg['uuid'] = object['uuid'];
			msg['isRoomHost'] = true;
			msg['count'] = 1;

			socketPool[object['uuid']]['userName'] = object['userName'];
			socketPool[object['uuid']]['count'] = 1;
			socketPool[object['uuid']]['msg'] = object['userName'] + ' 创建了房间';

			ws.send(JSON.stringify(msg));
		} else if (object['type'] == 'join') {
			if (socketPool[object['uuid']] == undefined) {
				sendError(ws, '房间不存在');
			} else {
				socketPool[object['uuid']]['msg'] = object['userName'] + ' 加入了房间';
				let msg = {};
				msg['type'] = 'init';
				msg['userName'] = socketPool[object['uuid']]['userName'];
				msg['timeStamp'] = new Date().getTime();
				msg['uuid'] = object['uuid'];
				msg['isRoomHost'] = false;
				getRealCnt();
				msg['count'] = ++socketPool[object['uuid']]['count'];
				ws.send(JSON.stringify(msg));
			}
		} else if (object['type'] == 'exit') {
			getRealCnt();
			if (!object['isRoomHost']) {
				socketPool[object['uuid']]['msg'] = object['userName'] + ' 退出了房间';
				delete socketPool[object['uuid']]['member'][object['userName']];
				socketPool[object['uuid']]['count']--;
			}
			let msg = {};
			msg['type'] = 'exit';
			ws.send(JSON.stringify(msg));

			setTimeout(function () {
				ws.close();
			}, 200);

		} else if (object['type'] == 'data') {

			try {
				if (object['isRoomHost'] == true) {
					socketPool[object['uuid']]['currentTime'] = object['currentTime'];
					socketPool[object['uuid']]['playbackRate'] = object['playbackRate'];
					socketPool[object['uuid']]['isPaused'] = object['isPaused'];
					socketPool[object['uuid']]['isEnded'] = object['isEnded'];
					socketPool[object['uuid']]['url'] = object['url'];
					socketPool[object['uuid']]['serverTime'] = object['serverTime'];
				}
				let msg = {};
	
				if (socketPool[object['uuid']] == undefined) {
					msg['type'] = 'error';
					msg['msg'] = '房主已断开连接';
					ws.send(JSON.stringify(msg));
					msg = { type: 'exit' };
				} else {
					if (!object['isRoomHost']) {
						socketPool[object['uuid']]['member'][object['userName']] = new Date().getTime();
					}
	
					msg['type'] = 'data';
					msg['timeStamp'] = new Date().getTime();
					msg['count'] = socketPool[object['uuid']]['count'];
					msg['msg'] = socketPool[object['uuid']]['msg'];
	
					if (!object['isRoomHost']) {
						msg['currentTime'] = socketPool[object['uuid']]['currentTime'];
						msg['currentTime'] = socketPool[object['uuid']]['currentTime'];
						msg['playbackRate'] = socketPool[object['uuid']]['playbackRate'];
						msg['isPaused'] = socketPool[object['uuid']]['isPaused'];
						msg['isEnded'] = socketPool[object['uuid']]['isEnded'];
						msg['url'] = socketPool[object['uuid']]['url'];
						msg['serverTime'] = socketPool[object['uuid']]['serverTime'];
					}
				}
				ws.send(JSON.stringify(msg));
			} catch {
				let msg = {};
				msg['type'] = 'exit';
				ws.send(JSON.stringify(msg));
				setTimeout(function () {
					ws.close();
				}, 200);
			}
		}

	});

	ws.on('close', function close() {
		console.log('Client disconnected');
	});
});

function sendError(ws, err = 'ERROR') {
	ws.send(JSON.stringify({ type: 'error', msg: err }));
}

function removeRoom() {
	for (i in socketPool) {
		if (new Date().getTime() - socketPool[i]['serverTime'] > 12000) {
			delete socketPool[i];
			console.log(`remove ${i}`);
		}
	}
}

function getRealCnt() {
	for (i in socketPool) {
		socketPool[i]['count'] = 1;
		for (j in socketPool[i]) {
			if (j != 'member') {
				continue;
			} else {
				for (k in socketPool[i][j]) {
					if (new Date().getTime() - socketPool[i][j][k] > 12000) {
						socketPool[i]['msg'] = k + ' 断开了连接';
						delete socketPool[i][j][k];
					} else {
						socketPool[i]['count']++;
						console.log('realCnt', socketPool[i]['count']);
					}
				}
			}
		}
	}
}
