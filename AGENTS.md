# AGENTS.md — happy-downloader

本文件对仓库内所有改动生效，优先级高于任何会话内的临时约定。

## 前端必须兼容 Chrome 109

**目标浏览器是 Windows 7 上的 Chrome 109**（Windows 7 能装的最后一个 Chrome），
以及荣耀畅玩50 等 Android 设备上的 Chromium 内核浏览器。用户明确要求：
所有前端代码都必须兼容 Chrome 109。

### 硬性要求

1. **不得使用 Chrome 110 及以后才有的 API。** 已知清单：

   | API | 起始版本 |
   |---|---|
   | `Array.prototype.toSorted` / `toReversed` / `toSpliced` / `with` | 110 |
   | `Object.groupBy` / `Map.groupBy` | 117 |
   | `Promise.withResolvers` | 119 |
   | `Promise.try` | 128 |
   | `Array.fromAsync` | 121 |
   | `String.prototype.isWellFormed` / `toWellFormed` | 111 |
   | `URL.canParse` / `URL.parse` | 120 / 126 |
   | `AbortSignal.any` | 116 |
   | `Set.prototype.union` / `intersection` / `difference` / `symmetricDifference` / `isSubsetOf` / `isSupersetOf` / `isDisjointFrom` | 122 |
   | `document.startViewTransition` | 111（react-router 已做特性检测，可用） |
   | `HTMLElement.showPopover` / `hidePopover` / `togglePopover` | 114 |
   | `scheduler.yield` | 129 |
   | `Intl.DurationFormat` | 129 |
   | `Element.moveBefore` | 133 |

   CSS 同理：`color-mix()`、`oklch()`、`oklab()`（111）、`light-dark()`（123）、
   `text-wrap: balance`（114）、`@scope`（118）、`@starting-style`（117）、
   `interpolate-size`（129）、`:user-valid` / `:user-invalid`（119）、
   `anchor positioning`（125）。

2. **构建目标固定为 `chrome87`**（见 `frontend/vite.config.mts`）。
   Vite 默认是 `baseline-widely-available`（= Chrome 111），会让打包器允许产出
   109 解析不了的语法。87 低于 109，是为了兼容停在更早版本、无法再更新的
   Windows 7 安装。

3. **`frontend/scripts/check-browser-support.mjs` 必须在两个 workflow 里都跑**，
   且新增 API 要同步加进脚本的检测表。它扫描打包产物，命中任何超过 109 的
   API 或 CSS 特性就让构建失败。语法层面的问题由 build target 负责，API 层面
   的问题只能靠这个脚本 —— 降级语法无法凭空造出一个不存在的 API。

4. **`frontend/package.json` 里没有 `version` 字段**，这是有意的：
   版本号只有 tag 一个来源，不要再加第二个。

### 新增代码时

- 写完跑 `pnpm exec tsc --noEmit` 和 `pnpm exec vitest run`。
- 引入新依赖前先确认它不引入 ES2021+ 语法或 110+ 的 API。
- 用 `Array.prototype.at()`（92）、`replaceAll()`（85）、`Object.hasOwn()`（93）、
  `structuredClone()`（98）、`crypto.randomUUID()`（92）、`findLast()`（97）、
  `Intl.Segmenter()`（87）是安全的。

## 相关文件

| 文件 | 作用 |
|---|---|
| `frontend/vite.config.mts` | 构建目标（语法层面） |
| `frontend/scripts/check-browser-support.mjs` | API 层面的浏览器兼容检查 |
| `.github/workflows/test.yml` | 在 backend job 里跑兼容检查 |
| `.github/workflows/release.yml` | 在发布前跑兼容检查 |
| `VERSIONING.md` | 版本号规则：PATCH 不需要审批，MINOR/MAJOR 需要 |
