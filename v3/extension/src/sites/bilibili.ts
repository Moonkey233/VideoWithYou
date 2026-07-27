import { createGenericAdapter } from "./generic";
import { IVideoAdapter, PlayerState } from "./types";

function extractBV(url: string): string {
  const match = url.match(/BV[0-9A-Za-z]+/);
  return match ? match[0] : "";
}

export function createBilibiliAdapter(): IVideoAdapter {
  const base = createGenericAdapter("bilibili");
  return {
    ...base,
    detect() {
      return location.hostname.includes("bilibili.com") ? 10 : 0;
    },
    readState(video: HTMLVideoElement): PlayerState {
      const state = base.readState(video);
      state.media.attrs = state.media.attrs || {};
      const bv = extractBV(location.href);
      if (bv) {
        state.media.attrs.bv = bv;
      }
      state.media.site = "bilibili";
      return state;
    }
  };
}