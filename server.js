const fs = require('fs');
const WebSocket = require('ws');
const https = require('https');
const port = 1206

const ca = `-----BEGIN CERTIFICATE-----\nMIIDazCCAlOgAwIBAgIUTASMYRLAkZ4LmtRwxETkME5IxhUwDQYJKoZIhvcNAQELBQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMzAzMjYwMTQwMTRaFw0zMzAzMjMwMTQwMTRaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDXVETZNw1lHoHDTOGFsEEiANFSxZRPfvQUsk1ZmLdu5vb1wdkgR0/7r5J3Bu2Q5ilzZVWgkv7Esge2P5o9SVLjFf+zZp+g/GNukHRDih7qiKqJ/NP/9pcsmyJ4O6zr6Q4mlK2FUR+lRCOTsZvA5ZvO1y+ggrGNVfDgqSrDE13ND9XloCVNO0v7R3SFiWW3iUYa1LVaBxkfnhedpMRX+kovs0ASsaL7agRWIAyVo5tNHDBu8UxG+M2/WwseO8Aa0YbfL8ixfZ69uN7/nWF283jUMHFc39ZXUane3nm88pUHWO/P1grqrlD/8MZeduHcQ9gAJJ/iCFi7Xalm3jHWY6z7AgMBAAGjUzBRMB0GA1UdDgQWBBT5t5c19GpAtSdLKylSCX+Hmq3wcTAfBgNVHSMEGDAWgBT5t5c19GpAtSdLKylSCX+Hmq3wcTAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQBtNbwpKoem3px4r23WDvAC0cBH46JMR4+liwC9zrULW4pVmdXR2NHHmhpCxgHcZb83NTJPE03YsOIAC3qesoErwQMc1lNM3wRWATzEPasJYdaYJz9nEwN4kBIUeLDjw03IeLNTNv/x4F6rkM/hKRKqpJWPYBbEXZyTEgXmBlpd6LT0EC6eV2PCwhR0RC7iuIo+m3q+rSceQlTJxyUpYab2ULFmKqHyAtgS/UIJT77Fdj5admDf+OypFpVBaqTJOxKU6xzpwQLeBU9rVatgIZHKP4Iscr93QkrMqMvM8NW1r0TSvfcJnzdUH38DQ7RtYvoOpGOZ0LdtXpWJIudJQcte\n-----END CERTIFICATE-----`;


const options = {
	key: fs.readFileSync('server.key'),
	cert: fs.readFileSync('server.crt'),
	//ca: fs.readFileSync('ca.crt'), // 指定自定义的CA证书
	ca: ca,
	rejectUnauthorized: false
  };

const server = https.createServer(options);
const wss = new WebSocket.Server({ server });
server.listen(port, () => {
	console.log('WSS server started');
  });
// const wss = new WebSocket.Server({port: port});
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

setInterval(removeRoom, 1000);
setInterval(getRealCnt, 2000);

wss.on('connection', function connection(ws) {
	console.log('Client connected');

	ws.on('message', function incoming(message) {
		console.log('received: %s', message);
		var object = JSON.parse(message)

		if (object['type'] == 'create') {
			socketPool[object['uuid']] = {member: {}};

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
		if (new Date().getTime() - socketPool[i]['serverTime'] >= 5000) {
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
					if (new Date().getTime() - socketPool[i][j][k] >= 2000) {
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