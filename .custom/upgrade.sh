#!/usr/bin/env bash
# .custom/upgrade.sh — 同步官方 new-api 更新，并刷新本地补丁记录
#
# 用法:  bash .custom/upgrade.sh
#
# 做三件事:
#   1. 拉取官方 (upstream) 最新代码，更新 main 分支
#   2. 把 custom 分支 rebase 到新 main 上（你的修改重放到最新代码）
#   3. 重新生成 .custom/patches/ 补丁文件（既是记录，也是兜底重放数据）
#
# 冲突处理: rebase 遇到冲突会停下。
#   - 解决冲突后:  git add <文件> && git rebase --continue
#   - 放弃本次:    git rebase --abort   (回到升级前状态，不会丢东西)
#
# 兜底（rebase 彻底乱套时，patch 还在）:
#   git checkout main
#   git checkout -B custom
#   git am .custom/patches/*.patch
# ---------------------------------------------------------------------------

set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# 颜色
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
say()  { echo -e "${GREEN}[upgrade]${NC} $*"; }
warn() { echo -e "${YELLOW}[upgrade]${NC} $*"; }
err()  { echo -e "${RED}[upgrade]${NC} $*" >&2; }

# 前置检查
if ! git remote get-url upstream >/dev/null 2>&1; then
  err "找不到 upstream 远程。请先: git remote add upstream https://github.com/QuantumNous/new-api.git"
  exit 1
fi
if ! git rev-parse --verify custom >/dev/null 2>&1; then
  err "找不到 custom 分支。"
  exit 1
fi

# 工作区必须干净，否则 rebase 风险大
if ! git diff-index --quiet HEAD --; then
  err "工作区有未提交改动，请先 commit 或 stash 再升级。"
  git status --short
  exit 1
fi

START_BRANCH="$(git branch --show-current)"

# 1. 拉取官方更新
say "拉取 upstream 最新代码..."
git fetch upstream

# 2. 更新 main
say "切换到 main，合并 upstream/main..."
git checkout main
if git merge --ff-only upstream/main; then
  say "main 已更新到最新。"
else
  err "main 与 upstream/main 出现分叉（可能 main 被直接写过）。请手动处理:"
  err "  git merge upstream/main   # 解决冲突后 commit"
  git checkout "$START_BRANCH" 2>/dev/null || true
  exit 1
fi

# 3. rebase custom 到新 main
say "切换到 custom，rebase 到 main..."
git checkout custom
if git rebase main; then
  say "rebase 成功，custom 已重放到最新代码上。"
else
  warn "rebase 遇到冲突，已暂停。请解决后执行:"
  warn "  git add <冲突文件>"
  warn "  git rebase --continue"
  warn "放弃本次升级: git rebase --abort"
  exit 1
fi

# 4. 刷新补丁记录
say "重新生成补丁文件到 .custom/patches/ ..."
mkdir -p .custom/patches
# 清理旧补丁，重新生成（补丁随 commit 内容变化，旧的无意义）
rm -f .custom/patches/*.patch
if git rev-list --count main..custom | grep -q '^0$'; then
  warn "custom 分支没有任何领先于 main 的 commit，未生成补丁。"
else
  git format-patch main..custom -o .custom/patches
  say "补丁已生成:"
  ls .custom/patches/*.patch 2>/dev/null | sed 's/^/    /'
fi

# 回到起始分支
if [ "$START_BRANCH" != "custom" ]; then
  git checkout "$START_BRANCH" 2>/dev/null || true
fi

say "升级完成。"
