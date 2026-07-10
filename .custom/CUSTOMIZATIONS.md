# 定制化修改清单 (CUSTOMIZATIONS)

> 本文件是**人类可读的修改记录**，与 `.custom/patches/` 下的 git 补丁互为补充。
> 每加一个功能，就在下面追加一节。升级时遇到冲突，先看这里定位"为什么改、改了哪里"。

## 约定原则

1. **新增文件为主**：尽量新建前端 page / component / feature，少改核心文件。
2. **核心文件只做注册**：必须改动核心文件时（如侧边栏注册、导航注册、i18n），只加一两行注册代码，并在下面记录。
3. **一个功能 = 一个 commit**：commit message 用 `feat(custom): xxx` 前缀，方便识别。
4. **后端基本不动**：unis-api 以 WEB 显示功能为主；若确需后端配合，新增 controller/model 文件，核心文件只加注册行。

## new-api 前端关键注册点（供参考，按需补充）

| 功能类型 | 涉及目录 | 注册入口 |
|---|---|---|
| 前端页面 | `web/default/src/features/<name>/` | 新建 feature + route 薄壳 |
| 路由薄壳 | `web/default/src/routes/_authenticated/<name>/index.tsx` | 新建即可，`routeTree.gen.ts` 自动生成纳入 |
| 侧边栏分区 | `web/default/src/hooks/use-sidebar-data.ts` | 在 sidebar 数据里加分区/项 |
| 顶部导航 | `web/default/src/hooks/use-top-nav-links.ts` | 推送导航链接 |
| 导航开关 | `web/default/src/features/system-settings/maintenance/config.ts` + `header-navigation-section.tsx` | 模块类型 + 默认值 + 开关 schema |
| i18n 文案 | `web/default/src/i18n/locales/{en,zh}.json` | 尾部追加 key（官方常改易冲突，单独 commit） |

> 注意：`web/default/src/routeTree.gen.ts` 是 TanStack Router 自动生成（带 `@ts-nocheck`），
> dev/build 时自动纳入新路由，**无需手改、不要手动提交其变更以外的干扰**。

## 镜像备份约定（可选）

新增文件可同步复制一份到 `.custom/ported/frontend/`（路径与源一致）作为兜底恢复源。
`.custom/ported/` 不进 git（冗余）。核心文件的修改以 git diff / patch 形式存在于 `.custom/patches/`。

---

## 修改记录

<!--
模板：每加一个功能复制下面这段，填好后追加到本节末尾。

### [N] 功能标题
- **Commit**: 见 patches/000N
- **日期**: YYYY-MM-DD
- **目的**: 一句话说明为什么做、解决什么。
- **新增文件**:
  - `web/default/src/features/xxx/index.tsx`
  - `web/default/src/routes/_authenticated/xxx/index.tsx`
- **修改的核心文件**:
  - `web/default/src/hooks/use-sidebar-data.ts` — 注册侧边栏项（+N 行）
  - `web/default/src/i18n/locales/en.json` + `zh.json` — 新 key（+N 行）
- **数据库**: 无（前端功能）/ 描述新表
- **依赖**: 无 / 依赖功能 [M]
- **回滚**: 删除新增文件 + 撤销核心文件对应行
-->

（在此追加你的修改记录）

---

## 升级冲突重点关注

按冲突概率从高到低（升级时优先核对这些文件）：

1. **`web/default/src/i18n/locales/en.json` / `zh.json`** — 官方常加 key，尾部追加易冲突
2. **`web/default/src/hooks/use-sidebar-data.ts`** — 官方可能调整侧边栏结构
3. **`web/default/src/hooks/use-top-nav-links.ts`** — 导航结构官方可能改
4. **`web/default/src/features/system-settings/maintenance/`** — 维护设置结构官方可能改

新增文件（features / routes / components）**零冲突**，升级时直接保留。
