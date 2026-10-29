# 渠道运行时监控（vLLM Running/Waiting）与调度联动

本模块把 vLLM 引擎的运行时并发数据接进网关调度层，解决三个问题：

1. **可观测**：定时采集每个渠道（= 一个 vLLM 实例）的 Running / Waiting / KV cache / 前缀命中率，落库成时序数据。
2. **饱和保护**：渠道排队了就别再往上压，锁一段时间只让存量请求排空。
3. **按输入长度分流**：短请求和长上下文请求走不同的渠道池，避免长请求打爆小 KV cache 的实例。

> 默认全部关闭（`enabled = false`）。没配置、没开启时，调度完全走原有逻辑，不会读运行时数据。

## 1. 数据流

```
渠道 base_url ──自动推导──► http://<host>/metrics
                                   │ (每 N 秒, 默认 10s)
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

默认保留 **12 个月（1 年）**，采集循环每 10 分钟清理一次过期数据（`sample_retention_months` 可调，上限 12 个月）。

> 数据量提醒：10s 间隔 × 单渠道 ≈ 8640 条/天、315 万条/年（1s 间隔则约 8.6 万条/天、3153 万条/年）。
> 渠道多或跑在 SQLite 上时，
> 建议把 `sample_retention_months` 调小（例如 1 个月）。
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
> **采集范围就是「最大 batch」表里列出的渠道**：没列进去的渠道不会发任何 `/metrics` 请求，
> 官方 API 这类没有 `/metrics` 的渠道只要不配置即可。

解析的指标：

| 指标 | 含义 |
| --- | --- |
| `vllm:num_requests_running` | Running |
| `vllm:num_requests_waiting` | Waiting |
| `vllm:max_num_seqs` | 引擎最大并发（vLLM 不一定暴露） |
| `vllm:kv_cache_usage_perc`（新版）/ `vllm:gpu_cache_usage_perc`（旧版） | KV cache 占用（0~1，入库乘 100）。vLLM ≥ 0.9 只暴露新名字，两个都要认 |
| `vllm:gpu_prefix_cache_hit_rate` / `vllm:prefix_cache_hits_total` | 前缀命中率（无 gauge 时用 hits/queries 计算） |

指标带 `model_name` 标签时按模型归属；不带标签时归属渠道的第一个模型。

## 4. 最大 batch（按渠道名配置，默认 4）

一个渠道 = 一个 vLLM 实例 = 一个模型，所以没有再绑定模型名。这张表同时决定**采集范围**：
列在表里的渠道才采集，没列的一个请求都不发。

优先级：

```
max_batch_overrides 中按 channel_name 命中  >  vllm:max_num_seqs  >  default_max_batch(默认 4)
```

后台打开配置页时会**默认把环境里的所有渠道列出来**，最大 batch 统一填 `default_max_batch`（4），
环境里一个渠道都没有时显示一行示例 `example-channel`。

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
2. `Waiting > 0` 且连续 `saturation_waiting_samples`（默认 3 次 = 3s）**不下降**
3. 该渠道所服务模型在同一分组内的候选渠道数 `> 1`

解锁条件：

- `Running < maxBatch` 且 `Waiting == 0`，连续 `saturation_release_samples`（默认 1 次）；或
- 锁定超过 `saturation_max_lock_seconds`（默认 300s）；或
- 采集连续失败约 6 个周期（metrics 挂了不该把渠道冻死）；或
- 后台手工点「解锁」（`POST /api/channel_runtime/:id/unlock`）。

选渠道时被锁的渠道会被跳过；**保护规则**：过滤后若一个候选都不剩，则忽略锁，宁可过载也不能无渠道可用。

## 6. 按输入长度分流

长度单位固定为 **估算 prompt token**（用请求体做 token 估算，不再提供字符数选项）。

后台是一张表：**模型名 + 渠道名 + 长度区间 [min, max)**，每行把一个模型的一个长度区间绑到单个渠道。
打开配置页时默认把环境里所有渠道列出来，模型名预填该渠道的第一个模型，区间默认 `[0, 0]`（无上限），
默认不启用——需要分流时把对应行打勾即可。

配置示例（`channel_runtime_setting.length_routing_rules`）：

```json
[
  { "model_name": "glm-5", "channel_name": "GLM5-H20-01", "min_length": 0,     "max_length": 10000, "enabled": true },
  { "model_name": "glm-5", "channel_name": "GLM5-A100-long", "min_length": 10000, "max_length": 0, "enabled": true }
]
```

- 区间为 `[min_length, max_length)`，`max_length = 0` 表示无上界，`[0, 0]` 即「全量兜底」。
- **下界最大的规则优先**：`[10000, 0]` 会盖住 `[0, 0]`，长请求自动走专用渠道。
- `model_name` 留空表示对所有模型生效；渠道名按名称解析成 id（解析结果缓存 60s，改名后一分钟内生效）。
- 命中规则后，该请求只能调到这一条渠道；亲和性命中的渠道不在集合内会被跳过
  （不会清掉亲和缓存，短请求仍能复用会话粘性）。
- `length_routing_fallback = true`（默认）：集合内没有可用渠道时退回全量候选，避免配置写错直接 503。
- 请求体超过 8MB 直接按"超长"处理，不做全文遍历。

### 长度怎么算（token 计数规则）

分流边界是**真实 prompt token 数**，所以计数方式必须贴近模型自己的 tokenizer：

| 模型 | 计数方式 |
| --- | --- |
| OpenAI 家族（`gpt-*`、`o1/o3/o4`、`chatgpt`） | 自己的 tiktoken 编码（o200k / cl100k）精确计数 |
| 其他模型（qwen、glm、deepseek 等，tokenkit 认不出家族） | 按文本语种选 **BPE 代理编码**：CJK 字符占比 ≥ 20% 用 o200k（`gpt-4o`），否则用 cl100k（`gpt-4`） |

> 为什么不用 `tokenkit.Estimate`：它是按字符权重累加的启发式（中文每字 0.85 token），
> 对 Qwen 这类分词器高估约 **1.7 倍**——实测 4800 个汉字：Estimate 4080，
> 两者的真实值见下表。旧的字符权重会让 2400 token 的请求算成 4100，
> 跨过 4096 的边界跑进长上下文渠道（.104 收到本该走 .204 的短请求）。
>
> 代理编码的选择依据（同一批请求，vLLM 返回的真实 `prompt_tokens` 做基准）：

| 样本 | 真实 token | `Estimate` | o200k | cl100k |
| --- | --- | --- | --- | --- |
| 4800 汉字 | 2408 | 4080（+69%） | **2400（0%）** | 3200（+33%） |
| 22500 字符英文 | 4507 | 5042（+12%） | 4000（−11%） | **4499（0%）** |

> 纯中文、纯英文都能精确命中；混合文本取其中一个编码，最坏偏差约 15%。

计数范围是**整个请求体**（含 JSON 结构，约几十 token 的固定开销），不计响应。

> 性能：BPE 计数是纯 Go 实现，约 1MB/s，所以只对前 **16KB** 做精确计数；
> 更大的请求体按「样本 token 数 × 总体积 / 样本体积」外推。
> 分流边界附近的请求体通常只有几 KB，能拿到精确值；远超边界的请求体只需要量级正确。

## 7. 配置项总表

配置整体存于 `options` 表，键前缀 `channel_runtime_setting.`，后台「系统管理 → 模型与路由 → 渠道运行时监控」可视化编辑。

| 键 | 默认 | 说明 |
| --- | --- | --- |
| `enabled` | false | 采集总开关；关闭后清空内存态并解除所有锁，调度回到原有逻辑 |
| `scrape_interval_seconds` | 10 | 采集间隔（1~300）。饱和锁的连读次数按这个间隔计数，10s × 3 = 约 30 秒才判锁 |
| `scrape_timeout_seconds` | 3 | 单次抓取超时（秒，1~30） |
| `sample_retention_months` | 12 | 时序数据保留时长（月，上限 12） |
| `default_max_batch` | 4 | 新增渠道行时预填的最大 batch |
| `max_batch_overrides` | `[]` | `[{channel_name, max_batch}]`，**同时是采集白名单** |
| `saturation_lock_enabled` | true | 饱和锁开关（采集未开启时不生效） |
| `saturation_waiting_samples` | 3 | 连读多少次排队才锁 |
| `saturation_release_samples` | 1 | 连读多少次空闲才解锁 |
| `saturation_max_lock_seconds` | 300 | 最长锁定时间，0 = 不限 |
| `length_routing_enabled` | false | 长度分流开关 |
| `length_routing_fallback` | true | 无可用渠道时回退全量 |
| `length_routing_rules` | `[]` | `[{model_name, channel_name, min_length, max_length, enabled}]` |

## 8. 接口

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/channel_runtime/status` | channel read | 各渠道当前 Running/Waiting/最大 batch/锁定状态 |
| GET | `/api/channel_runtime/samples?channel_id=&model_name=&hours=&limit=` | channel read | 时序快照 |
| GET | `/api/channel_runtime/limits` | channel read | 最大 batch 解析结果 |
| GET | `/api/channel_runtime/channels` | channel read | 环境中的全部渠道（配置页默认行用，不含密钥） |
| GET | `/api/channel_runtime/series?start_timestamp=&end_timestamp=&bucket_seconds=` | channel read | 看板用：按时间桶聚合的 Running/Waiting/最大 batch/吞吐（含 `avg_requests` 均值与 `max_requests` 峰值）+ 渠道清单 |
| GET | `/api/channel_runtime/usage_series?start_timestamp=&end_timestamp=&bucket_seconds=` | channel read | 看板用：按渠道 × 时间桶聚合的 `logs` 调用量 |
| GET | `/api/channel_runtime/resource?hours=24` | channel read | 旧版接口，保留但前端不再使用 |
| POST | `/api/channel_runtime/refresh` | channel operate | 立即采集一次 |
| POST | `/api/channel_runtime/:id/unlock` | channel operate | 手工解锁 |

## 9. 看板：渠道资源分析

位置：数据看板 → **渠道资源分析**（`/dashboard/channels`，仅管理员可见）。

页面从上到下三块，共用顶部的时间范围（当天 / 当周 / 当月 / 自定义）：

- **当前运行时**：每个渠道一行，展示 Running / Waiting / 最大 batch / KV% / 前缀命中率 / 状态 / 采集时间，
  每 10 秒自动刷新；右侧「刷新状态」会立即触发一次采集，并让下方「渠道调用分析」与每渠道趋势一起重新拉取
  （采集是异步落库的，按钮触发后 2 秒再刷新一次）。
- **渠道调用分析**：复用模型调用分析的趋势 / 分布 / 排行三张图。
  数据来源是 `logs` 表的计费日志（`type=2`，按 `channel_id` 聚合），**只有经过网关的请求才会被统计**；
  直连 vLLM 端口的压测流量不计入。
- **每渠道趋势卡片**（调用并发数 / Prefill 性能 / Decode 性能）：
  - 「调用并发数」是组合图：Running 曲线到 0 轴之间的区域按利用率着色 —— 空闲（`<50%`）绿、
    正常（`≥50%`）蓝、繁忙（`≥80%`）黄、饱和（`≥100%`）红，与旧版拥挤度色标一致；
    另叠加「最大 batch」与「峰值在途请求」两条参考线。
  - 「峰值在途请求」= 该时间桶内 `Running + Waiting` 的**最大值**（`max_requests`），是整数观测值。
    早先用的是同一时间桶内的平均值（`avg_requests`），把多个瞬时采样平均后会出现
    `7.83` 这类小数——那是在途请求数的**均值**，不是请求个数，看起来像"半个请求"。
  - 「Prefill 性能 / Decode 性能」各一条吞吐曲线，单位 tokens/s。

## 10. 部署注意

- **多节点**：各节点独立采集、独立判定锁（锁是内存态，跟本地流量视图一致）；样本表靠唯一索引去重，
  不会因为多节点重复写而膨胀。若节点间行为差异较大，可通过关闭某节点上的采集开关来收敛。
- **vLLM 侧**：`/metrics` 必须可从网关容器访问（同内网/同 compose 网络），不要挂在需要鉴权的网关后面。
- **指标缺失**：早期 vLLM 没有 `vllm:max_num_seqs`，此时用 `max_batch_overrides` 按渠道名配置，
  或直接依赖默认值 4。
- **长上下文分流与亲和性**：两者同时开启时，长度分流优先；被分流排除的亲和渠道只跳过、不清缓存。
