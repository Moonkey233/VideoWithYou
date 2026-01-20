const DEFAULT_CLIENT_HOST = "127.0.0.1";
const DEFAULT_CLIENT_PORT = 27111;
const DEFAULT_CLIENT_PATH = "/ext";

let socket: WebSocket | null = null;
let lastTabId: number | null = null;
let activeTabId: number | null = null;
let activeWindowId: number | null = null;
let lastConnectAttempt = 0;
const pending: string[] = [];
let clientPort = DEFAULT_CLIENT_PORT;
let portReady = false;
let portLoading = false;
let pendingConnect = false;
let lastInRoom = false;
let lastServerConnected: boolean | null = null;
let lastNoticeKey = "";
let lastNoticeAt = 0;

function notifyClientStatus(connected: boolean) {
  if (!connected) {
    lastInRoom = false;
    lastServerConnected = null;
    lastNoticeKey = "";
    lastNoticeAt = 0;
  }
  chrome.runtime.sendMessage({
    type: "client_status",
    payload: { connected }
  });
}

function shouldNotify(key: string): boolean {
  const now = Date.now();
  if (key && key === lastNoticeKey && now - lastNoticeAt < 5000) {
    return false;
  }
  lastNoticeKey = key;
  lastNoticeAt = now;
  return true;
}

function showNotify(message: string) {
  if (!message) return;
  forwardToTab({
    type: "notify",
    payload: { message }
  });
}

function isRoomClosedError(message: string): boolean {
  const lower = message.toLowerCase();
  return lower.includes("room closed") || lower.includes("host left");
}

function isExtensionIdleError(message: string): boolean {
  const lower = message.toLowerCase();
  return lower.includes("extension idle");
}

function handleUiState(state: any) {
  const role = typeof state?.role === "string" ? state.role : "";
  const inRoom = role === "host" || role === "follower";
  const serverConnected =
    typeof state?.server_connected === "boolean" ? state.server_connected : null;
  const error = typeof state?.last_error === "string" ? state.last_error.trim() : "";
  const roomClosed = error ? isRoomClosedError(error) : false;

  if (!lastInRoom && inRoom) {
    lastNoticeKey = "";
    lastNoticeAt = 0;
  }

  if (lastInRoom && !inRoom && error) {
    if (!isExtensionIdleError(error)) {
      if (roomClosed && shouldNotify("room_closed")) {
        showNotify("\u623f\u4e3b\u5df2\u9000\u51fa, \u623f\u95f4\u5df2\u89e3\u6563");
      } else if (!roomClosed && shouldNotify(`room_error:${error}`)) {
        showNotify(`\u623f\u95f4\u5df2\u7ed3\u675f: ${error}`);
      }
    }
  } else if (inRoom && lastServerConnected === true && serverConnected === false) {
    if (shouldNotify("server_disconnected")) {
      showNotify("\u670d\u52a1\u5668\u8fde\u63a5\u4e2d\u65ad, \u8bf7\u68c0\u67e5\u7f51\u7edc");
    }
  }

  lastInRoom = inRoom;
  lastServerConnected = serverConnected;
}

function normalizePort(value: unknown): number | null {
  const port = Number.parseInt(String(value), 10);
  if (!Number.isFinite(port) || port < 1 || port > 65535) {
    return null;
  }
  return port;
}

function getWsUrl() {
  return `ws://${DEFAULT_CLIENT_HOST}:${clientPort}${DEFAULT_CLIENT_PATH}`;
}

function loadClientPort() {
  if (portReady || portLoading) {
    return;
  }
  portLoading = true;
  chrome.storage.local.get({ client_port: DEFAULT_CLIENT_PORT }, (data) => {
    const port = normalizePort(data.client_port);
    clientPort = port ?? DEFAULT_CLIENT_PORT;
    portReady = true;
    portLoading = false;
    if (pendingConnect) {
      pendingConnect = false;
      connectSocket();
    }
  });
}

function setClientPort(value: unknown) {
  const port = normalizePort(value);
  if (port === null || port === clientPort) {
    return;
  }
  clientPort = port;
  portReady = true;
  portLoading = false;
  chrome.storage.local.set({ client_port: port });
  if (socket) {
    try {
      socket.close();
    } catch {
      // ignore close errors
    }
  }
  socket = null;
  notifyClientStatus(false);
  connectSocket();
}

function connectSocket() {
  if (!portReady) {
    pendingConnect = true;
    loadClientPort();
    return;
  }
  const now = Date.now();
  if (socket && (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING)) {
    return;
  }
  if (now - lastConnectAttempt < 1500) {
    return;
  }
  lastConnectAttempt = now;

  try {
    socket = new WebSocket(getWsUrl());
  } catch (err) {
    console.warn("local client ws connect failed", err);
    socket = null;
    notifyClientStatus(false);
    return;
  }

  socket.onopen = () => {
    notifyClientStatus(true);
    while (pending.length > 0 && socket && socket.readyState === WebSocket.OPEN) {
      const payload = pending.shift();
      if (payload) {
        socket.send(payload);
      }
    }
  };

  socket.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data as string);
      handleClientMessage(msg);
    } catch {
      // ignore invalid payloads
    }
  };

  socket.onclose = () => {
    socket = null;
    notifyClientStatus(false);
  };

  socket.onerror = () => {
    notifyClientStatus(false);
  };
}

function enqueue(payload: string) {
  pending.push(payload);
  if (pending.length > 50) {
    pending.shift();
  }
}

function sendToClient(msg: any) {
  if (!msg || !msg.type) return;
  const payload = JSON.stringify(msg);

  if (!socket || socket.readyState !== WebSocket.OPEN) {
    if (msg.type !== "player_state" && msg.type !== "ext_hello" && msg.type !== "ext_ping") {
      enqueue(payload);
    }
    connectSocket();
    return;
  }

  socket.send(payload);
}

function handleClientMessage(msg: any) {
  if (!msg || !msg.type) return;
  if (msg.type === "apply_state" || msg.type === "navigate") {
    forwardToTab(msg);
    return;
  }
  if (msg.type === "ui_state") {
    handleUiState(msg.payload);
    chrome.runtime.sendMessage(msg);
    return;
  }
  if (msg.type === "room_event") {
    chrome.runtime.sendMessage(msg);
  }
}

function forwardToTab(msg: any) {
  const targetTab = msg.payload?.tabId ?? activeTabId ?? lastTabId;
  if (typeof targetTab === "number") {
    chrome.tabs.sendMessage(targetTab, msg);
    return;
  }
  chrome.tabs.query({ active: true, lastFocusedWindow: true }, (tabs) => {
    if (tabs[0]?.id) {
      chrome.tabs.sendMessage(tabs[0].id, msg);
    }
  });
}

function refreshActiveTab(windowId?: number) {
  const query: chrome.tabs.QueryInfo = { active: true };
  if (typeof windowId === "number") {
    query.windowId = windowId;
  } else {
    query.lastFocusedWindow = true;
  }
  chrome.tabs.query(query, (tabs) => {
    const tab = tabs[0];
    if (tab?.id) {
      activeTabId = tab.id;
      if (typeof tab.windowId === "number") {
        activeWindowId = tab.windowId;
      }
    }
  });
}

function isFromActiveTab(sender: chrome.runtime.MessageSender): boolean {
  const tab = sender.tab;
  if (!tab || typeof tab.id !== "number") {
    return false;
  }
  if (activeWindowId !== null && typeof tab.windowId === "number" && tab.windowId !== activeWindowId) {
    return false;
  }
  if (tab.active) {
    activeTabId = tab.id;
    if (typeof tab.windowId === "number") {
      activeWindowId = tab.windowId;
    }
    return true;
  }
  return activeTabId !== null && tab.id === activeTabId;
}

chrome.runtime.onMessage.addListener((msg, sender) => {
  if (!msg || !msg.type) return;
  if (msg.type === "set_client_port") {
    setClientPort(msg.payload?.port);
    return;
  }

  const fromTab = Boolean(sender.tab?.id);
  if (fromTab) {
    const isStateMsg = msg.type === "player_state" || msg.type === "ext_hello";
    const fromActive = isFromActiveTab(sender);
    if (isStateMsg && !fromActive) {
      return;
    }
    if (fromActive && sender.tab?.id) {
      lastTabId = sender.tab.id;
    }
  }

  sendToClient(msg);
});

chrome.tabs.onActivated.addListener((info) => {
  activeTabId = info.tabId;
  activeWindowId = info.windowId;
});

chrome.tabs.onRemoved.addListener((tabId) => {
  if (tabId === activeTabId) {
    activeTabId = null;
  }
  if (tabId === lastTabId) {
    lastTabId = null;
  }
});

chrome.tabs.onUpdated.addListener((tabId, _changeInfo, tab) => {
  if (tab.active && (activeWindowId === null || tab.windowId === activeWindowId)) {
    activeTabId = tabId;
    if (typeof tab.windowId === "number") {
      activeWindowId = tab.windowId;
    }
  }
});

chrome.windows.onFocusChanged.addListener((windowId) => {
  if (windowId === chrome.windows.WINDOW_ID_NONE) {
    activeWindowId = null;
    return;
  }
  activeWindowId = windowId;
  refreshActiveTab(windowId);
});

chrome.windows.getLastFocused((win) => {
  if (win?.id) {
    activeWindowId = win.id;
  }
  refreshActiveTab(win?.id);
});

loadClientPort();
