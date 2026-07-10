# .custom/ — unis-api (new-api 二次开发) 工具包

本目录是 unis-api 的**定制化工具包**，服务于一个目标：
**在 fork new-api 的基础上加自己的功能（以 WEB 显示功能为主），同时能随时升级官方版本而不丢改动。**

> 参考实现：`G:/AI/new-api-dev/.custom`（同一套机制）。

## 目录结构

```
.custom/
├── README.md             ← 本文件（随仓库提交，开源可见）
├── CUSTOMIZATIONS.md     ← 人类可读的修改清单（每加一个功能记一笔）
├── upgrade.sh            ← 一键升级脚本
├── patches/              ← git 自动生成的补丁（升级时自动刷新，本地兜底，不进 git）
└── ported/               ← 新增文件的镜像备份（兜底恢复源，不进 git）
```

## 两个分支

| 分支 | 用途 | 规则 |
|---|---|---|
| `main` | 跟踪官方 upstream | **纯净**，绝不写自定义代码 |
| `custom` | 你的二次开发（即 unis-api 的产品形态） | 每个功能 = 一个独立 commit |

> 远程 `upstream` 指向官方 `QuantumNous/new-api`。
> 远程 `origin` 指向 unis-api 自己的 GitHub 仓库（对外发布用）。
> **开源用户克隆的是 `custom` 分支**（建议在 GitHub 上把 `custom` 设为默认分支）。

## 与参考实现的差异（开源适配）

参考 `new-api-dev` 把整个 `.custom/` 放进 `.git/info/exclude`（本地私有）。
unis-api 作为开源项目，**提交工具包本身**（`upgrade.sh` / `CUSTOMIZATIONS.md` / `README.md`），
让社区能复现升级流程；只把**可再生产物**排除在 git 外：

| 路径 | 是否进 git | 原因 |
|---|---|---|
| `.custom/upgrade.sh` | ✅ 提交 | 升级流程，开源可见 |
| `.custom/CUSTOMIZATIONS.md` | ✅ 提交 | 修改清单，开源可见 |
| `.custom/README.md` | ✅ 提交 | 说明 |
| `.custom/patches/*.patch` | ❌ 忽略 | 升级时由 `upgrade.sh` 自动重新生成 |
| `.custom/ported/` | ❌ 忽略 | 已提交文件的镜像，冗余 |

> 若你更想要参考那种「整个 .custom 本地私有」的形态：
> 把 `.custom/` 整行加入 `.git/info/exclude`，并改用 `git format-patch` 在本地兜底即可。

## 日常工作流

### 写自定义功能时

```bash
git checkout custom
# ... 写代码（新增文件为主，核心文件只加注册行）...
git add <相关文件>
git commit -m "feat(custom): 描述你的功能"
# 在 CUSTOMIZATIONS.md 追加一节记录
# 刷新补丁记录（可选，upgrade.sh 也会做）
git format-patch main..custom -o .custom/patches
```

### 升级官方版本时

```bash
bash .custom/upgrade.sh
```

脚本自动：① 拉取 upstream ② 更新 main ③ rebase custom ④ 刷新补丁。
无冲突则全自动；有冲突则停在 rebase，按提示解决后 `git rebase --continue`。

## 兜底方案

rebase 彻底失败时，补丁文件还在，可从干净状态重放：

```bash
git checkout main
git checkout -B custom        # 重置 custom 到 main
git am .custom/patches/*.patch # 逐个重放你的修改
```

## 冲突最小化原则

升级时冲突只出现在"你改过、官方也改过"的同一行。所以：

- ✅ 新建文件：page / component / feature —— 零冲突
- ✅ 核心文件只加一两行注册代码 —— 冲突点固定、好解决
- ❌ 大段改写核心文件 —— 冲突多、难维护

把每次改动记到 `CUSTOMIZATIONS.md`，升级时能快速定位"为什么改、改了哪里"。
