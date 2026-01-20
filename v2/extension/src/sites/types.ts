export interface MediaInfo {
  url: string;
  title: string;
  site: string;
  attrs: Record<string, string>;
}

export interface PlayerState {
  position_ms: number;
  duration_ms: number;
  paused: boolean;
  rate: number;
  media: MediaInfo;
}

export interface ApplyState {
  position_ms: number;
  paused: boolean;
  rate: number;
}

export interface IVideoAdapter {
  name: string;
  detect(): number;
  attach(): HTMLVideoElement | null;
  readState(video: HTMLVideoElement): PlayerState;
  applyState(video: HTMLVideoElement, state: ApplyState): Promise<void>;
  observe(video: HTMLVideoElement, onChange: () => void): void;
}