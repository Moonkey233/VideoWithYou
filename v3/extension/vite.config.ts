import { defineConfig } from "vite";
import { resolve } from "path";

export default defineConfig({
  build: {
    outDir: "dist",
    emptyOutDir: true,
    rollupOptions: {
      input: {
        popup: resolve(__dirname, "popup.html"),
        sw: resolve(__dirname, "src/sw.ts"),
        content: resolve(__dirname, "src/content.ts")
      },
      output: {
        entryFileNames: (chunk) => {
          if (chunk.name === "sw") return "sw.js";
          if (chunk.name === "content") return "content.js";
          return "assets/[name].js";
        }
      }
    }
  }
});