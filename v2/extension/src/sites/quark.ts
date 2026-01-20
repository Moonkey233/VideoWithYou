import { createGenericAdapter } from "./generic";
import { IVideoAdapter } from "./types";

export function createQuarkAdapter(): IVideoAdapter {
  const base = createGenericAdapter("quark");
  return {
    ...base,
    detect() {
      return location.hostname.includes("quark") ? 9 : 0;
    }
  };
}