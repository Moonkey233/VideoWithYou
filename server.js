const { json } = require('stream/consumers');
const WebSocket = require('ws');
const fs = require('fs');

const port = 1206

const wss = new WebSocket.Server({port: port});
console.log(`已开始监听: 0.0.0.0: ${port}`)

var socketPool = {};

// class MyWebSocket extends WebSocket {
// 	constructor(brand) {
// 		super(brand);
// 	}
// 	sendError(err = 'ERROR') {
// 		let data = {type: 'error', msg: err};
// 		this.send(JSON(stringify(data)));
// 	}
// }

wss.on('connection', function connection(ws) {
	console.log('Client connected');

	ws.on('message', function incoming(message) {
		console.log('received: %s', message);
		var object = JSON.parse(message)

		if (object['type'] == 'create') {
			socketPool[object['uuid']] = {};
			let msg = {};
			msg['type'] = 'init';
			msg['userName'] = object['userName'];
			msg['serverTime'] = new Date().getTime();
			msg['uuid'] = object['uuid'];
			msg['isRoomHost'] = true;
			msg['count'] = 1;

			socketPool[object['uuid']]['userName'] = object['userName'];
			socketPool[object['uuid']]['count'] = 1;

			ws.send(JSON.stringify(msg));
		} else if (object['type'] == 'join') {
			if (socketPool[object['uuid']] == undefined) {
				sendError(ws, '房间不存在');
			} else {
				let msg = {};
				msg['type'] = 'init';
				msg['userName'] = socketPool[object['uuid']]['userName'];
				msg['timeStamp'] = new Date().getTime();
				msg['uuid'] = object['uuid'];
				msg['isRoomHost'] = false;
				msg['count'] = ++socketPool[object['uuid']]['count'];
				ws.send(JSON.stringify(msg));
			}
		} else if (object['type'] == 'exit') {
			socketPool[object['uuid']]['count'] -= 1;
			if (object['isRoomHost'] == true) {
				delete socketPool[object['uuid']];
			}
			let msg = {};
			msg['type'] = 'exit';
			ws.send(JSON.stringify(msg));

			setTimeout(function () {
				ws.close();
			}, 200);

		} else if (object['type'] == 'data') {
			if (object['isRoomHost'] == true) {
				socketPool[object['uuid']]['currentTime'] = object['currentTime'];
				socketPool[object['uuid']]['playbackRate'] = object['playbackRate'];
				socketPool[object['uuid']]['isPaused'] = object['isPaused'];
				socketPool[object['uuid']]['isEnded'] = object['isEnded'];
				socketPool[object['uuid']]['bvid'] = object['bvid'];
				socketPool[object['uuid']]['serverTime'] = object['serverTime'];
			}

			let msg = {};
			msg['type'] = 'data';
			msg['timeStamp'] = new Date().getTime();
			msg['count'] = socketPool[object['uuid']]['count'];
			if (!object['isRoomHost']) {
				msg['currentTime'] = socketPool[object['uuid']]['currentTime'];
				msg['currentTime'] = socketPool[object['uuid']]['currentTime'];
				msg['playbackRate'] = socketPool[object['uuid']]['playbackRate'];
				msg['isPaused'] = socketPool[object['uuid']]['isPaused'];
				msg['isEnded'] = socketPool[object['uuid']]['isEnded'];
				msg['bvid'] = socketPool[object['uuid']]['bvid'];
				msg['serverTime'] = socketPool[object['uuid']]['serverTime'];
			}

			ws.send(JSON.stringify(msg));
		}

	});

	ws.on('close', function close() {
		console.log('Client disconnected');
	});
});

function sendError(ws, err = 'ERROR') {
	ws.send(JSON.stringify({ type: 'error', msg: err }));
}