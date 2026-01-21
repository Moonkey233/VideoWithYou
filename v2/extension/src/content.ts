import { createBilibiliAdapter } from "./sites/bilibili";
import { createQuarkAdapter } from "./sites/quark";
import { createGenericAdapter } from "./sites/generic";
import { ApplyState, IVideoAdapter } from "./sites/types";

const adapters: IVideoAdapter[] = [
  createBilibiliAdapter(),
  createQuarkAdapter(),
  createGenericAdapter("generic")
];

const EXT_PING_INTERVAL_MS = 3000;

let currentAdapter: IVideoAdapter = adapters[adapters.length - 1];
let currentVideo: HTMLVideoElement | null = null;

function pickAdapter(): IVideoAdapter {
  let best = adapters[adapters.length - 1];
  let bestScore = -1;
  for (const adapter of adapters) {
    const score = adapter.detect();
    if (score > bestScore) {
      bestScore = score;
      best = adapter;
    }
  }
  return best;
}

function attachVideo() {
  currentAdapter = pickAdapter();
  currentVideo = currentAdapter.attach();
  if (currentVideo) {
    currentAdapter.observe(currentVideo, sendPlayerState);
    sendPlayerState();
  }
  sendHello();
}

function sendHello() {
  chrome.runtime.sendMessage({
    type: "ext_hello",
    payload: {
      url: location.href,
      site: currentAdapter.name,
      version: "v2"
    }
  });
}

function sendPlayerState() {
  if (!currentVideo) return;
  const state = currentAdapter.readState(currentVideo);
  chrome.runtime.sendMessage({ type: "player_state", payload: state });
}

async function applyState(state: ApplyState) {
  if (!currentVideo) {
    attachVideo();
  }
  if (!currentVideo) return;
  await currentAdapter.applyState(currentVideo, state);
}

function handleMessage(msg: any) {
  if (!msg || !msg.type) return;
  if (msg.type === "apply_state") {
    applyState(msg.payload as ApplyState);
    return;
  }
  if (msg.type === "navigate") {
    const target = msg.payload?.url;
    if (typeof target === "string" && target.length > 0 && target !== location.href) {
      location.href = target;
    }
    return;
  }
  if (msg.type === "notify") {
    const message = msg.payload?.message;
    if (typeof message === "string" && message.trim()) {
      window.alert(message);
    }
  }
}

function watchRouteChanges() {
  const notify = () => setTimeout(attachVideo, 200);
  const pushState = history.pushState;
  const replaceState = history.replaceState;

  history.pushState = function (...args) {
    pushState.apply(history, args);
    notify();
  };
  history.replaceState = function (...args) {
    replaceState.apply(history, args);
    notify();
  };
  window.addEventListener("popstate", notify);

  const observer = new MutationObserver(() => {
    if (!currentVideo || currentVideo.isConnected === false) {
      attachVideo();
    }
  });
  observer.observe(document.documentElement, { childList: true, subtree: true });
}

chrome.runtime.onMessage.addListener(handleMessage);
attachVideo();
watchRouteChanges();
setInterval(sendPlayerState, 1000);
setInterval(() => {
  chrome.runtime.sendMessage({ type: "ext_ping" });
}, EXT_PING_INTERVAL_MS);
