# 定制化修改清单 (CUSTOMIZATIONS)

> **2026-10-28 上游 rc.47 合并说明**：上游把 `web/default/`、`web/classic/` 合并到 `web/` 根目录，
> 定制前端代码已同步迁移到 `web/src/`（原 `web/default/src/`）。同时上游新增本地子模块
> `relaykit/`（dto、relayconvert、types）与 `tokenkit/`（token 估算），渠道选择重构为
> `dto.ChannelFilter` / `dto.ChannelConstraints` + `service.SelectChannelForRequest`。
> 本文档中的路径已按新结构更新。


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
| 前端页面 | `web/src/features/<name>/` | 新建 feature + route 薄壳 |
| 路由薄壳 | `web/src/routes/_authenticated/<name>/index.tsx` | 新建即可，`routeTree.gen.ts` 自动生成纳入 |
| 侧边栏分区 | `web/src/hooks/use-sidebar-data.ts` | 在 sidebar 数据里加分区/项 |
| 顶部导航 | `web/src/hooks/use-top-nav-links.ts` | 推送导航链接 |
| 导航开关 | `web/src/features/system-settings/maintenance/config.ts` + `header-navigation-section.tsx` | 模块类型 + 默认值 + 开关 schema |
| i18n 文案 | `web/src/i18n/locales/{en,zh}.json` | 尾部追加 key（官方常改易冲突，单独 commit） |

> 注意：`web/src/routeTree.gen.ts` 是 TanStack Router 自动生成（带 `@ts-nocheck`），
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
  - `web/src/features/xxx/index.tsx`
  - `web/src/routes/_authenticated/xxx/index.tsx`
- **修改的核心文件**:
  - `web/src/hooks/use-sidebar-data.ts` — 注册侧边栏项（+N 行）
  - `web/src/i18n/locales/en.json` + `zh.json` — 新 key（+N 行）
- **数据库**: 无（前端功能）/ 描述新表
- **依赖**: 无 / 依赖功能 [M]
- **回滚**: 删除新增文件 + 撤销核心文件对应行
-->

（在此追加你的修改记录）

---

## 初始移植（移植自 MIXAPI / new-api-dev，2026-07-10）

以下 6 个功能从 `new-api-dev` 工作区移植，按功能边界拆成 6 个 commit。
**注意**：i18n 的 34 个新 key 为一整块尾部追加，为减少升级冲突点，
统一并入 [5] 提交（而非参考实现的 [5]+[6] 各分一半）。[6] 因此不含 i18n 改动。

### [1] usage_statistics 汇总表 + rollup 写入
- **Commit**: 0001
- **新增**: `model/usage_statistics.go`
- **核心文件**: `model/main.go` AutoMigrate(+1)，`model/log.go` RecordConsumeLog/RecordTaskBillingLog 调 rollup
- **数据库**: 新增 `usage_statistics` 表（三库兼容 Upsert）
- **回滚**: 删新增文件 + 撤 main.go/log.go 对应行

### [2] Claude 用户输入提取 + Log.UserInput 列 + 开关
- **Commit**: 0002
- **新增**: `model/user_input_extract.go`
- **核心文件**: `model/log.go`(Log.UserInput 列+提取), `common/constants.go`(LogUserInputEnabled), `model/option.go`(option 注册), `relay/claude_handler.go` + `relay/channel/claude/relay-claude.go`(注入 claude_request)
- **数据库**: `logs` 表新增 `user_input` 列
- **依赖**: 无
- **回滚**: 删新增文件 + 撤核心文件对应行

### [3] 用量统计 API（日/月/rank + summary + 导出）+ 路由
- **Commit**: 0003
- **新增**: `controller/usage_statistics.go`, `model/usage_statistics_rank.go`
- **核心文件**: `router/api-router.go` — usage_statistics / _monthly / _rank 路由组
- **注意**: rank 查询 MySQL 专用，SQLite/PostgreSQL 下报错（已接受）
- **回滚**: 删新增文件 + 撤路由组

### [4] token 分布统计 API（prompt/completion/request_count）+ 路由
- **Commit**: 0004
- **新增**: `controller/token_distribution.go`, `model/token_distribution.go`
- **核心文件**: `router/api-router.go` — 3 个 distribution 路由组
- **依赖**: 功能 [2] 的 `logs.user_input` 列
- **注意**: 分布查询 MySQL 专用（已接受）
- **回滚**: 删新增文件 + 撤路由组

### [5] /info 公开页 + token-info 明文端点 + 导航注册 + i18n
- **Commit**: 0005
- **新增**: `web/src/features/info/{api.ts,index.tsx}`, `web/src/routes/info/index.tsx`
- **核心文件**: `controller/token.go`(GetAllTokensPlaintext), `model/token.go`(GetAllTokens/CountAllTokens), `router/api-router.go`(token-info 组), `nav-modules.ts`+`use-top-nav-links.ts`+maintenance `config.ts`/`header-navigation-section.tsx`(导航开关), `i18n/locales/{en,zh}.json`(34 key)
- **注意**: token 明文泄露为 /info 固有特性，由用户明确接受
- **回滚**: 删新增文件 + 撤核心文件对应行

### [6] 6 个统计页前端（日/月/rank + 3 分布）+ 侧边栏分区
- **Commit**: 0006
- **新增**: `web/src/features/usage-statistics/`(api+index+6 components), 6 个 `routes/_authenticated/*/index.tsx` 薄壳
- **核心文件**: `web/src/hooks/use-sidebar-data.ts` — Statistics 侧边栏分区(6 项)
- **依赖**: 后端 [3]+[4] 的 API 端点；i18n key 见 [5]
- **注意**: `routeTree.gen.ts` 自动生成，无需手改
- **回滚**: 删新增文件 + 撤 sidebar 分区

---

### [7] 渠道运行时监控（vLLM Running/Waiting）+ 饱和锁 + 长度路由 + 渠道资源分析看板
- **Commit**: 0009
- **新增**:
  - `setting/operation_setting/channel_runtime_setting.go` — 配置结构（默认关闭）
  - `model/channel_runtime.go` — 3 张表：`channel_runtime_samples`（时间序列）、`channel_runtime_limits`（渠道最大 batch）、`channel_runtime_state`（锁状态）
  - `service/channel_runtime_monitor.go` — 默认每 1s 抓取渠道 base_url 同源 `/metrics`，落库 Running/Waiting/max_batch，并按阈值上锁/解锁
  - `service/channel_length_routing.go` — 按 prompt tokens 阈值把请求约束到指定渠道
  - `controller/channel_runtime.go` — 状态/样本/上限/刷新/解锁/资源统计/渠道清单 7 个 API
  - `web/src/features/dashboard/components/channels/` — 看板「渠道资源分析」（调用量 + 拥挤度）
  - `web/src/features/system-settings/general/channel-runtime/` — 配置页
  - `docs/channel-runtime-monitor.md` — 设计与排障文档
- **核心文件**: `main.go`（启动采集器）、`model/main.go`（AutoMigrate）、`router/api-router.go`（`/api/channel_runtime/*` 路由组）、`middleware/distributor.go` + `service/channel_select.go` + `model/channel_cache.go` + `model/ability.go` + `constant/context_key.go`（长度路由约束）、`features/dashboard/{api.ts,index.tsx,section-registry.tsx}`、`features/system-settings/{types.ts,models/section-registry.tsx}`、`i18n/locales/zh.json`
- **依赖**: vLLM 侧暴露 `vllm:num_requests_running/num_requests_waiting`；max_batch 优先级 渠道名手工配置 > `max_num_seqs` > 默认 4
- **注意**: `enabled` 默认 false，不配置即完全走原有调度逻辑；样本保留默认 12 个月（SQLite 部署建议调小到 1 个月）
- **二次优化（同日）**:
  - 采集超时改 **秒**（`scrape_timeout_seconds`，默认 3，1~30）；样本保留改 **月**（`sample_retention_months`，默认 12，上限 12）
  - 删除 `scrape_only_channels` 白名单：**采集范围 = 最大 batch 表里列出的渠道**（`IsChannelCollected`），移出范围时清内存锁状态
  - 配置页默认铺出环境全部渠道（`GET /api/channel_runtime/channels`，不含 key），最大 batch 默认 4；无渠道时展示 `example-channel` 示例行
  - 长度分离改为扁平表 `{model_name, channel_name, min_length, max_length, enabled}`，区间 `[0,0]` 表示无上限，**下界最大者优先**；渠道名→id 解析带 60s TTL 缓存
  - 看板「按渠道调用情况」去掉「额度」列（后端不再聚合 quota）
  - 旧键 `scrape_timeout_ms` / `sample_retention_hours` / `scrape_only_channels` / 旧 `length_routing_rules`（regex+segments 格式）升级后失效，需重新配置
- **三次优化（看板重构）**:
  - 新增采集 `vllm:avg_prompt_throughput_toks_per_s` / `vllm:avg_generation_throughput_toks_per_s`，落库字段 `prompt_throughput` / `generation_throughput`；实例未暴露这两个 gauge 时（本机 vLLM v0.32 即如此），用 `vllm:prompt_tokens_total` / `vllm:generation_tokens_total` 两次采集的增量估算 tokens/s
  - 新增 API：`GET /api/channel_runtime/series`（按时间桶聚合 Running/Waiting/max_batch/prefill/decode 吞吐 + 渠道清单）、`GET /api/channel_runtime/usage_series`（按渠道 + 时间桶聚合 logs 调用量）
  - 配置页更名为「渠道运行时配置」，只保留配置项；「当前运行时」表格（含刷新采集、解锁）移到看板「渠道资源分析」
  - 看板「渠道资源分析」改为：时间范围（当天 / 当周 / 当月 / 自定义）+ 当前运行时 + 渠道调用分析（复用模型调用分析的调用趋势 / 调用次数分布 / 调用次数排行）+ 每渠道趋势卡片（调用并发数 / Prefill 性能 / Decode 性能，吞吐单位 tokens/s）
  - `/api/channel_runtime/resource` 保留但前端不再使用
- **回滚**: 删新增文件 + 停用 `main.go` 中 `StartChannelRuntimeMonitor()` + 撤路由组与 AutoMigrate 注册

---

## 升级冲突重点关注

按冲突概率从高到低（升级时优先核对这些文件）：

1. **`web/src/i18n/locales/en.json` / `zh.json`** — 官方常加 key，尾部追加易冲突
2. **`web/src/hooks/use-sidebar-data.ts`** — 官方可能调整侧边栏结构
3. **`web/src/hooks/use-top-nav-links.ts`** — 导航结构官方可能改
4. **`web/src/features/system-settings/maintenance/`** — 维护设置结构官方可能改

新增文件（features / routes / components）**零冲突**，升级时直接保留。
