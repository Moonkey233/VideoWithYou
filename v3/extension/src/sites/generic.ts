import { ApplyState, IVideoAdapter, PlayerState } from "./types";

function pickBestVideo(): HTMLVideoElement | null {
  const videos = Array.from(document.querySelectorAll("video"));
  let best: HTMLVideoElement | null = null;
  let bestArea = 0;

  for (const video of videos) {
    const rect = video.getBoundingClientRect();
    const area = rect.width * rect.height;
    if (area <= 0) continue;
    if (!isVisible(rect)) continue;
    if (area > bestArea) {
      best = video;
      bestArea = area;
    }
  }
  return best;
}

function isVisible(rect: DOMRect) {
  return rect.width > 0 && rect.height > 0;
}

export function createGenericAdapter(siteName = "generic"): IVideoAdapter {
  return {
    name: siteName,
    detect() {
      return 1;
    },
    attach() {
      return pickBestVideo();
    },
    readState(video: HTMLVideoElement): PlayerState {
      return {
        position_ms: Math.floor(video.currentTime * 1000),
        duration_ms: Number.isFinite(video.duration) ? Math.floor(video.duration * 1000) : 0,
        paused: video.paused,
        rate: video.playbackRate || 1,
        media: {
          url: location.href,
          title: document.title,
          site: siteName,
          attrs: {
            video_src: video.currentSrc || ""
          }
        }
      };
    },
    async applyState(video: HTMLVideoElement, state: ApplyState): Promise<void> {
      if (state.rate && Math.abs(video.playbackRate - state.rate) > 0.001) {
        video.playbackRate = state.rate;
      }

      if (state.position_ms >= 0) {
        const target = state.position_ms / 1000;
        if (Math.abs(video.currentTime - target) > 0.05) {
          video.currentTime = target;
        }
      }

      if (state.paused) {
        if (!video.paused) {
          video.pause();
        }
      } else if (video.paused) {
        try {
          await video.play();
        } catch {
          // Ignore autoplay failures.
        }
      }
    },
    observe(video: HTMLVideoElement, onChange: () => void) {
      const handler = () => onChange();
      video.addEventListener("timeupdate", handler);
      video.addEventListener("play", handler);
      video.addEventListener("pause", handler);
      video.addEventListener("ratechange", handler);
      video.addEventListener("seeking", handler);
    }
  };
}