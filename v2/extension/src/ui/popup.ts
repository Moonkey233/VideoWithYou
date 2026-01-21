const statusEl = document.getElementById("status") as HTMLParagraphElement;
const roomLabelPrefixEl = document.getElementById("roomLabelPrefix") as HTMLSpanElement;
const roomCodeEl = document.getElementById("roomCode") as HTMLSpanElement;
const membersEl = document.getElementById("membersCount") as HTMLSpanElement;
const errorEl = document.getElementById("error") as HTMLDivElement;
const eventsEl = document.getElementById("roomEvents") as HTMLDivElement;
const lastSyncEl = document.getElementById("lastSync") as HTMLSpanElement;
const roleBadge = document.getElementById("roleBadge") as HTMLSpanElement;
const endpointBadge = document.getElementById("endpointBadge") as HTMLSpanElement;
const joinCodeEl = document.getElementById("joinCode") as HTMLInputElement;
const displayNameEl = document.getElementById("displayName") as HTMLInputElement;
const preRoomEl = document.getElementById("preRoom") as HTMLDivElement;
const inRoomEl = document.getElementById("inRoom") as HTMLDivElement;
const copyBtn = document.getElementById("copyBtn") as HTMLButtonElement;
const followUrlEl = document.getElementById("followUrl") as HTMLInputElement;
const clientPortEl = document.getElementById("clientPort") as HTMLInputElement;
const applyPortBtn = document.getElementById("applyPortBtn") as HTMLButtonElement;

const createBtn = document.getElementById("createBtn") as HTMLButtonElement;
const joinBtn = document.getElementById("joinBtn") as HTMLButtonElement;
const leaveBtn = document.getElementById("leaveBtn") as HTMLButtonElement;

let localConnected = false;
let serverConnected: boolean | null = null;
let currentRoomCode = "";
let copyTimeout: number | undefined;
const copyLabel = copyBtn.textContent || "复制房间号";
const defaultClientPort = 27111;
let currentClientPort = defaultClientPort;
const roomEvents: string[] = [];
const maxEvents = 6;
const eventSuffixes = ["进入了房间", "离开了房间"];

function scrollEventsToBottom() {
  window.requestAnimationFrame(() => {
    eventsEl.scrollTop = eventsEl.scrollHeight;
  });
}

function updateStatus() {
  if (!localConnected) {
    statusEl.textContent = "本地客户端未连接";
    return;
  }
  if (serverConnected === true) {
    statusEl.textContent = "本地客户端已连接, 服务器已连接";
    return;
  }
  if (serverConnected === false) {
    statusEl.textContent = "本地客户端已连接, 服务器未连接";
    return;
  }
  statusEl.textContent = "本地客户端已连接";
}

function formatRole(value: unknown): string {
  switch (value) {
    case "host":
      return "房主";
    case "follower":
      return "成员";
    default:
      return "-";
  }
}

function formatEndpoint(value: unknown): string {
  switch (value) {
    case "browser":
      return "在线浏览器";
    case "mpc":
      return "本地播放器 (MPC-BE)";
    default:
      return "-";
  }
}

function localizeError(value: unknown): string {
  if (typeof value !== "string") {
    return "";
  }
  const trimmed = value.trim();
  if (!trimmed) {
    return "";
  }
  const lower = trimmed.toLowerCase();
  if (lower === "already in a room") {
    return "已经在房间中";
  }
  if (lower === "room action pending") {
    return "房间操作处理中";
  }
  if (lower === "nickname required") {
    return "请先填写昵称";
  }
  if (lower === "room not found") {
    return "房间不存在";
  }
  if (lower === "room closed (host left)") {
    return "房间已解散 (房主离开)";
  }
  if (lower.includes("room closed")) {
    return "房间已解散";
  }
  return trimmed;
}

function sendAction(action: string, payload: Record<string, unknown> = {}) {
  chrome.runtime.sendMessage({
    type: "ui_action",
    payload: {
      action,
      ...payload
    }
  });
}

function renderEvents() {
  eventsEl.innerHTML = "";
  for (const entry of roomEvents) {
    const div = document.createElement("div");
    const parsed = splitEventMessage(entry);
    if (parsed && parsed.name) {
      const nameSpan = document.createElement("span");
      nameSpan.className = "event-name";
      nameSpan.textContent = parsed.name;
      const actionSpan = document.createElement("span");
      actionSpan.className = "event-action";
      actionSpan.textContent = ` ${parsed.action}`;
      div.append(nameSpan, actionSpan);
    } else {
      div.textContent = entry;
    }
    eventsEl.appendChild(div);
  }
  scrollEventsToBottom();
}

function splitEventMessage(message: string): { name: string; action: string } | null {
  const trimmed = message.trim();
  if (!trimmed) {
    return null;
  }
  for (const suffix of eventSuffixes) {
    if (trimmed.endsWith(suffix)) {
      const name = trimmed.slice(0, trimmed.length - suffix.length).trim();
      return { name, action: suffix };
    }
  }
  return null;
}

function appendEvent(message: string) {
  if (!message) return;
  roomEvents.push(message);
  if (roomEvents.length > maxEvents) {
    roomEvents.shift();
  }
  renderEvents();
}

function clearEvents() {
  roomEvents.length = 0;
  renderEvents();
}

function setEventsFromState(events: unknown) {
  if (!Array.isArray(events)) {
    return;
  }
  roomEvents.length = 0;
  for (const entry of events) {
    if (typeof entry === "string") {
      roomEvents.push(entry);
    }
  }
  if (roomEvents.length > maxEvents) {
    roomEvents.splice(0, roomEvents.length - maxEvents);
  }
  renderEvents();
}

function normalizePort(value: string): number | null {
  const port = Number.parseInt(value, 10);
  if (!Number.isFinite(port) || port < 1 || port > 65535) {
    return null;
  }
  return port;
}

function setClientPort(port: number) {
  currentClientPort = port;
  clientPortEl.value = String(port);
}

createBtn.addEventListener("click", () => {
  const name = displayNameEl.value.trim();
  sendAction("create_room", { display_name: name });
});
joinBtn.addEventListener("click", () => {
  const code = joinCodeEl.value.trim();
  const name = displayNameEl.value.trim();
  if (code) {
    sendAction("join_room", { room_code: code, display_name: name });
  }
});
leaveBtn.addEventListener("click", () => sendAction("leave_room"));
copyBtn.addEventListener("click", () => {
  if (currentRoomCode) {
    navigator.clipboard.writeText(currentRoomCode).catch(() => {
      // ignore clipboard failure
    });
    copyBtn.textContent = "已复制";
    copyBtn.classList.add("copied");
    copyBtn.disabled = true;
    if (copyTimeout) {
      window.clearTimeout(copyTimeout);
    }
    copyTimeout = window.setTimeout(() => {
      copyBtn.textContent = copyLabel;
      copyBtn.classList.remove("copied");
      copyBtn.disabled = false;
    }, 3000);
  }
});

for (const input of document.querySelectorAll<HTMLInputElement>("input[name='endpoint']")) {
  input.addEventListener("change", () => {
    if (input.checked) {
      sendAction("set_endpoint", { endpoint: input.value });
    }
  });
}

followUrlEl.addEventListener("change", () => {
  sendAction("set_follow_url", { follow_url: followUrlEl.checked });
});

applyPortBtn.addEventListener("click", () => {
  const port = normalizePort(clientPortEl.value.trim());
  if (port === null) {
    setClientPort(currentClientPort);
    return;
  }
  if (port === currentClientPort) {
    return;
  }
  currentClientPort = port;
  chrome.runtime.sendMessage({
    type: "set_client_port",
    payload: { port }
  });
});

clientPortEl.addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    applyPortBtn.click();
  }
});

chrome.runtime.onMessage.addListener((msg) => {
  if (!msg || !msg.type) return;
  if (msg.type === "client_status") {
    localConnected = Boolean(msg.payload?.connected);
    if (!localConnected) {
      serverConnected = null;
      preRoomEl.hidden = false;
      inRoomEl.hidden = true;
      createBtn.disabled = false;
      joinBtn.disabled = false;
      if (copyTimeout) {
        window.clearTimeout(copyTimeout);
        copyTimeout = undefined;
      }
      copyBtn.textContent = copyLabel;
      copyBtn.classList.remove("copied");
      copyBtn.disabled = false;
      clearEvents();
      lastSyncEl.textContent = "-";
      roomLabelPrefixEl.textContent = "房间号: ";
      roomCodeEl.textContent = "-";
      membersEl.textContent = "-";
    }
    updateStatus();
    return;
  }
  if (msg.type === "room_event") {
    appendEvent(String(msg.payload?.message || ""));
    return;
  }
  if (msg.type !== "ui_state") return;
  const state = msg.payload || {};
  localConnected = true;
  serverConnected = typeof state.server_connected === "boolean" ? state.server_connected : null;
  updateStatus();

  const code = state.room_code || "";
  currentRoomCode = code;
  const displayName = state.display_name || "";
  const hostDisplayName = state.host_display_name || "";
  if (!displayNameEl.value && displayName) {
    displayNameEl.value = displayName;
  }
  const roomOwnerName = hostDisplayName || displayName;
  if (code) {
    const prefix = roomOwnerName ? `${roomOwnerName}的房间号: ` : "房间号: ";
    roomLabelPrefixEl.textContent = prefix;
    roomCodeEl.textContent = code;
  } else {
    roomLabelPrefixEl.textContent = "房间号: ";
    roomCodeEl.textContent = "-";
  }
  const membersCount = typeof state.members_count === "number" ? state.members_count : "-";
  membersEl.textContent = String(membersCount);
  roleBadge.textContent = `角色: ${formatRole(state.role)}`;
  endpointBadge.textContent = `模式: ${formatEndpoint(state.endpoint)}`;
  errorEl.textContent = localizeError(state.last_error);
  lastSyncEl.textContent = state.last_sync_time || "-";
  setEventsFromState(state.room_events);

  const role = state.role;
  const inRoom = role === "host" || role === "follower";
  preRoomEl.hidden = inRoom;
  inRoomEl.hidden = !inRoom;
  createBtn.disabled = inRoom;
  joinBtn.disabled = inRoom;
  if (!inRoom) {
    if (copyTimeout) {
      window.clearTimeout(copyTimeout);
      copyTimeout = undefined;
    }
    copyBtn.textContent = copyLabel;
    copyBtn.classList.remove("copied");
    copyBtn.disabled = false;
    clearEvents();
    lastSyncEl.textContent = "-";
  }

  if (state.endpoint) {
    const endpoint = state.endpoint;
    const radios = document.querySelectorAll<HTMLInputElement>("input[name='endpoint']");
    radios.forEach((radio) => {
      radio.checked = radio.value === endpoint;
    });
  }
  if (typeof state.follow_url === "boolean") {
    followUrlEl.checked = state.follow_url;
  }
});

setClientPort(defaultClientPort);
chrome.storage.local.get({ client_port: defaultClientPort }, (data) => {
  const port = normalizePort(String(data.client_port));
  setClientPort(port ?? defaultClientPort);
});

preRoomEl.hidden = false;
inRoomEl.hidden = true;
createBtn.disabled = false;
joinBtn.disabled = false;
updateStatus();
sendAction("refresh_state");
