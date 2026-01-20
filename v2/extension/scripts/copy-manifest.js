import { copyFileSync, mkdirSync } from "fs";
import { resolve } from "path";

const distDir = resolve("dist");
mkdirSync(distDir, { recursive: true });
copyFileSync(resolve("manifest.json"), resolve(distDir, "manifest.json"));