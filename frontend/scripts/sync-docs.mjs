// 将 docs/API_GUIDE.md 复制到 public/api-guide.md，供 API 文档页面运行时 fetch 渲染。
// 该命令已挂在 dev / build 启动前（package.json 的 predev / prebuild），无需手动执行。
import { copyFile, mkdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const src = resolve(here, "../../docs/API_GUIDE.md");
const dest = resolve(here, "../public/api-guide.md");

try {
  await mkdir(dirname(dest), { recursive: true });
  await copyFile(src, dest);
  console.log(`[sync-docs] 已复制 API 文档: ${src} -> ${dest}`);
} catch (err) {
  console.error(`[sync-docs] 复制 API 文档失败: ${err.message}`);
  process.exit(1);
}
