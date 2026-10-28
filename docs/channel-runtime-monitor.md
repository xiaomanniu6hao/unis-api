# 渠道运行时监控（vLLM Running/Waiting）与调度联动

本模块把 vLLM 引擎的运行时并发数据接进网关调度层，解决三个问题：

1. **可观测**：定时采集每个渠道（= 一个 vLLM 实例）的 Running / Waiting / KV cache / 前缀命中率，落库成时序数据。
2. **饱和保护**：渠道排队了就别再往上压，锁一段时间只让存量请求排空。
3. **按输入长度分流**：短请求和长上下文请求走不同的渠道池，避免长请求打爆小 KV cache 的实例。

> 默认全部关闭（`enabled = false`）。没配置、没开启时，调度完全走原有逻辑，不会读运行时数据。

## 1. 数据流

```
渠道 base_url ──自动推导──► http://<host>/metrics
                                   │ (每 N 秒, 默认 5s)
                                   ▼
                        采集器 service/channel_runtime_monitor.go
                                   │
                ┌──────────────────┼───────────────────┐
                ▼                  ▼                   ▼
   channel_runtime_samples   channel_runtime_limits   内存态 map[channelId]State
   (时序快照, 默认保留1年)     (每渠道最大 batch)        │
                                                          ▼
                                          model.filterChannelIdsByRuntimeState
                                                          │
                                          GetRandomSatisfiedChannel / GetChannel
```

采集与判定全在网关侧完成，不依赖 vLLM 日志文本；`APIServer` 日志里的
`Running: 4 reqs, Waiting: 0 reqs` 对应 `/metrics` 的 `vllm:num_requests_running`、
`vllm:num_requests_waiting`。

## 2. 表结构

| 表 | 主键 | 用途 |
| --- | --- | --- |
| `channel_runtime_samples` | 自增 id + 唯一索引 `(channel_id, model_name, collected_at)` | 每 N 秒一条时序快照，`collected_at` 按采集间隔对齐，多节点重复写入自动忽略 |
| `channel_runtime_limits` | `channel_id` | 每渠道解析出的最大 batch、来源、历史峰值 Running |

`channel_runtime_samples` 字段：`running`、`waiting`、`max_batch`、`kv_cache_usage`（0~100）、
`prefix_cache_hit_rate`（0~100）、`locked`（采样时刻是否处于饱和锁）。

默认保留 **8760 小时（1 年）**，采集循环每 10 分钟清理一次过期数据（`sample_retention_hours` 可调，上限 1 年）。

> 数据量提醒：5s 间隔 × 单渠道 ≈ 1.7 万条/天、630 万条/年。渠道多或跑在 SQLite 上时，
> 建议把 `sample_retention_hours` 调小（例如 720 = 30 天）。
>
> 若曾按旧版（主键为 `channel_id + model_name`）建过 `channel_runtime_limits`，先执行：
> `DROP TABLE channel_runtime_limits;` 让程序按新结构重建。

## 3. 采集地址（全自动）

采集地址完全由渠道 `base_url` 推导，**没有手工配置项**：

```
base_url  ->  scheme://host + /metrics
```

例：`http://10.210.139.118:8000/v1` → `http://10.210.139.118:8000/metrics`；
`https://glm5.internal/api/v1/chat` → `https://glm5.internal/metrics`。

> `/metrics` 是 vLLM 的固定标准路径，业务路径（`/v1/...`）会被忽略。
> 若某渠道不该被采集（例如官方 API 渠道没有 `/metrics`），用 `scrape_only_channels` 白名单排除。

解析的指标：

| 指标 | 含义 |
| --- | --- |
| `vllm:num_requests_running` | Running |
| `vllm:num_requests_waiting` | Waiting |
| `vllm:max_num_seqs` | 引擎最大并发（vLLM 不一定暴露） |
| `vllm:gpu_cache_usage_perc` | KV cache 占用（0~1，入库乘 100） |
| `vllm:gpu_prefix_cache_hit_rate` / `vllm:prefix_cache_hits_total` | 前缀命中率（无 gauge 时用 hits/queries 计算） |

指标带 `model_name` 标签时按模型归属；不带标签时归属渠道的第一个模型。

## 4. 最大 batch（按渠道配置，默认 4）

一个渠道 = 一个 vLLM 实例 = 一个模型，所以没有再绑定模型名。优先级：

```
max_batch_overrides 中按 channel_name 命中  >  vllm:max_num_seqs  >  default_max_batch(默认 4)
```

配置示例（`channel_runtime_setting.max_batch_overrides`）：

```json
[
  { "channel_name": "GLM5-H20-01", "max_batch": 32 },
  { "channel_name": "GLM5-H20-02", "max_batch": 16 }
]
```

历史峰值记录在 `channel_runtime_limits.peak_running`，只增不减，可用于判断真实上限：

```sql
SELECT channel_id, channel_name, max_batch, source, peak_running FROM channel_runtime_limits;
UPDATE channel_runtime_limits SET peak_running = 0 WHERE channel_id = 1;
```

## 5. 饱和锁

锁定条件（同时满足）：

1. `Running >= max_batch`
2. `Waiting > 0` 且连续 `saturation_waiting_samples`（默认 3 次 = 15s）**不下降**
3. 该渠道所服务模型在同一分组内的候选渠道数 `> 1`

解锁条件：

- `Running < maxBatch` 且 `Waiting == 0`，连续 `saturation_release_samples`（默认 1 次）；或
- 锁定超过 `saturation_max_lock_seconds`（默认 300s）；或
- 采集连续失败约 6 个周期（metrics 挂了不该把渠道冻死）；或
- 后台手工点「解锁」（`POST /api/channel_runtime/:id/unlock`）。

选渠道时被锁的渠道会被跳过；**保护规则**：过滤后若一个候选都不剩，则忽略锁，宁可过载也不能无渠道可用。

## 6. 按输入长度分流

长度单位固定为 **估算 prompt token**（用请求体做 token 估算，不再提供字符数选项）。

配置示例（`channel_runtime_setting.length_routing_rules`）：

```json
[
  {
    "name": "glm5 long context split",
    "model_regex": ["^GLM5.*$"],
    "enabled": true,
    "segments": [
      { "min_length": 0,     "max_length": 10000, "channel_ids": [1] },
      { "min_length": 10000, "max_length": 0,     "channel_ids": [2, 3, 4] }
    ]
  }
]
```

- 区间为 `[min_length, max_length)`，`max_length = 0` 表示无上界。
- 命中规则后，该请求只能调到 `channel_ids` 里的渠道；亲和性命中的渠道不在集合内会被跳过
  （不会清掉亲和缓存，短请求仍能复用会话粘性）。
- `length_routing_fallback = true`（默认）：集合内没有可用渠道时退回全量候选，避免配置写错直接 503。
- 请求体超过 8MB 直接按"超长"处理，不做全文遍历。

## 7. 配置项总表

配置整体存于 `options` 表，键前缀 `channel_runtime_setting.`，后台「系统管理 → 模型与路由 → 渠道运行时监控」可视化编辑。

| 键 | 默认 | 说明 |
| --- | --- | --- |
| `enabled` | false | 采集总开关；关闭后清空内存态并解除所有锁，调度回到原有逻辑 |
| `scrape_interval_seconds` | 5 | 采集间隔（1~300） |
| `scrape_timeout_ms` | 3000 | 单次抓取超时 |
| `sample_retention_hours` | 8760 | 时序数据保留时长（1 年，上限 1 年） |
| `scrape_only_channels` | `[]` | 白名单，非空时只采集这些渠道 |
| `default_max_batch` | 4 | 未匹配覆盖时的最大 batch |
| `max_batch_overrides` | `[]` | `[{channel_name, max_batch}]` |
| `saturation_lock_enabled` | true | 饱和锁开关（采集未开启时不生效） |
| `saturation_waiting_samples` | 3 | 连读多少次排队才锁 |
| `saturation_release_samples` | 1 | 连读多少次空闲才解锁 |
| `saturation_max_lock_seconds` | 300 | 最长锁定时间，0 = 不限 |
| `length_routing_enabled` | false | 长度分流开关 |
| `length_routing_fallback` | true | 无可用渠道时回退全量 |
| `length_routing_rules` | 示例规则 | 见上 |

## 8. 接口

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/channel_runtime/status` | channel read | 各渠道当前 Running/Waiting/最大 batch/锁定状态 |
| GET | `/api/channel_runtime/samples?channel_id=&model_name=&hours=&limit=` | channel read | 时序快照 |
| GET | `/api/channel_runtime/limits` | channel read | 最大 batch 解析结果 |
| GET | `/api/channel_runtime/resource?hours=24` | channel read | 看板用：按渠道聚合的调用情况 + 拥挤程度 |
| POST | `/api/channel_runtime/refresh` | channel operate | 立即采集一次 |
| POST | `/api/channel_runtime/:id/unlock` | channel operate | 手工解锁 |

## 9. 看板：渠道资源分析

位置：数据看板 → **渠道资源分析**（`/dashboard/channels`，仅管理员可见）。

- **第一块「按渠道调用情况」**：渠道 × 时间窗（24h / 7d）的请求数、流量占比、错误率、额度、平均耗时。
  数据来源 `logs` 表按 `channel_id` 聚合（成功计费日志 `type=2` + 错误日志 `type=5`）。
- **第二块「渠道拥挤程度」**：按利用率（`Running / max_batch`）从高到低排序，每条渠道一根进度条，
  ≥50% 蓝、≥80% 黄、≥100% 或已锁定红；右侧同时给出 `Running / max_batch`、`Waiting`、
  `KV%` 与状态徽章（空闲/正常/繁忙/饱和/无数据）。每 10 秒自动刷新。

## 10. 部署注意

- **多节点**：各节点独立采集、独立判定锁（锁是内存态，跟本地流量视图一致）；样本表靠唯一索引去重，
  不会因为多节点重复写而膨胀。若节点间行为差异较大，可用 `scrape_only_channels` 只让一个节点采集。
- **vLLM 侧**：`/metrics` 必须可从网关容器访问（同内网/同 compose 网络），不要挂在需要鉴权的网关后面。
- **指标缺失**：早期 vLLM 没有 `vllm:max_num_seqs`，此时用 `max_batch_overrides` 按渠道名配置，
  或直接依赖默认值 4。
- **长上下文分流与亲和性**：两者同时开启时，长度分流优先；被分流排除的亲和渠道只跳过、不清缓存。
