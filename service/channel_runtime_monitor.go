package service

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	metricNumRequestsRunning = "vllm:num_requests_running"
	metricNumRequestsWaiting = "vllm:num_requests_waiting"
	metricMaxNumSeqs         = "vllm:max_num_seqs"
	// vLLM 新版本（>= 0.9）把 KV cache 使用率改名为 vllm:kv_cache_usage_perc，
	// 旧版本仍是 vllm:gpu_cache_usage_perc，两个名字都要识别。
	metricKVCacheUsage            = "vllm:kv_cache_usage_perc"
	metricGPUCacheUsage           = "vllm:gpu_cache_usage_perc"
	metricPrefixCacheHitRate      = "vllm:gpu_prefix_cache_hit_rate"
	metricPrefixCacheHits         = "vllm:prefix_cache_hits_total"
	metricPrefixCacheQueries      = "vllm:prefix_cache_queries_total"
	metricAvgPromptThroughput     = "vllm:avg_prompt_throughput_toks_per_s"
	metricAvgGenerationThroughput = "vllm:avg_generation_throughput_toks_per_s"
	// 部分 vLLM 版本不暴露 avg_*_throughput gauge，只用累计 token 计数器，
	// 此时用两次采集间的增量估算吞吐（tokens/s）。
	metricPromptTokensTotal     = "vllm:prompt_tokens_total"
	metricGenerationTokensTotal = "vllm:generation_tokens_total"

	// runtimeCleanupInterval 控制过期时序数据的清理频率
	runtimeCleanupInterval = 10 * time.Minute
	// runtimeFailureLogInterval 限制同一渠道采集失败日志的输出频率
	runtimeFailureLogInterval = 2 * time.Minute
	// runtimeMetricsPath 是 vLLM 暴露 Prometheus 指标的标准路径。
	// 采集地址由渠道 base_url 推导（scheme://host + 该路径），不需要手工配置。
	runtimeMetricsPath = "/metrics"
)

var (
	channelRuntimeMonitorOnce sync.Once
	// channelRuntimeCollecting 防止手动刷新与定时轮次并发执行
	channelRuntimeCollecting atomic.Bool
	// channelRuntimeManualTrigger 手动立即采集信号
	channelRuntimeManualTrigger = make(chan struct{}, 1)
	// channelRuntimeLastFailureLog 记录每个渠道上次失败日志时间，避免刷屏
	channelRuntimeLastFailureLog sync.Map // channelId -> int64
	channelRuntimeLastCleanup    atomic.Int64
	// channelRuntimeCounters 保存上次采集的累计 token 数，用于估算吞吐
	channelRuntimeCounters sync.Map // "channelId/modelName" -> channelRuntimeCounter
)

// StartChannelRuntimeMonitor 启动运行时指标采集循环。
// 采集间隔读取配置，配置热更新后下一轮自动生效。
func StartChannelRuntimeMonitor() {
	channelRuntimeMonitorOnce.Do(func() {
		common.SysLog("channel runtime monitor started")
		go func() {
			for {
				setting := operation_setting.GetChannelRuntimeSetting()
				operation_setting.NormalizeChannelRuntimeSetting(setting)
				interval := time.Duration(setting.ScrapeIntervalSeconds) * time.Second
				if !setting.Enabled {
					interval = 30 * time.Second
					model.ResetChannelRuntimeStates()
				}

				select {
				case <-time.After(interval):
				case <-channelRuntimeManualTrigger:
				}

				if setting.Enabled {
					runChannelRuntimeCollection()
				}
			}
		}()
	})
}

// TriggerChannelRuntimeCollection 请求一次立即采集（后台手动刷新）。
func TriggerChannelRuntimeCollection() {
	select {
	case channelRuntimeManualTrigger <- struct{}{}:
	default:
	}
}

func runChannelRuntimeCollection() {
	if !channelRuntimeCollecting.CompareAndSwap(false, true) {
		return
	}
	defer channelRuntimeCollecting.Store(false)

	setting := operation_setting.GetChannelRuntimeSetting()
	operation_setting.NormalizeChannelRuntimeSetting(setting)
	if !setting.Enabled {
		return
	}

	channelIds, err := model.GetEnabledChannelIds()
	if err != nil {
		common.SysError("channel runtime: list channels failed: " + err.Error())
		return
	}

	interval := int64(setting.ScrapeIntervalSeconds)
	collectedAt := time.Now().Unix() / interval * interval

	var samples []model.ChannelRuntimeSample
	var limits []model.ChannelRuntimeLimit

	for _, channelId := range channelIds {
		channel, err := model.CacheGetChannel(channelId)
		if err != nil || channel == nil {
			continue
		}
		// 采集范围 = 后台配了最大 batch 的渠道；没配的渠道不产生任何请求，保持原有调度逻辑。
		// 移出采集范围时要顺手清掉内存态，否则历史锁定会让该渠道一直被跳过。
		if !setting.IsChannelCollected(channel.Name) {
			model.ClearChannelRuntimeState(channel.Id)
			continue
		}
		channelSamples, channelLimits, scrapeErr := collectChannelRuntime(channel, setting, collectedAt)
		if scrapeErr != nil {
			model.RecordChannelRuntimeError(channelId, runtimeMetricsURL(channel), scrapeErr, collectedAt)
			logChannelRuntimeFailure(channelId, scrapeErr)
			continue
		}
		samples = append(samples, channelSamples...)
		limits = append(limits, channelLimits...)
		applyChannelRuntimeObservation(channel, channelSamples, channelLimits, collectedAt)
	}

	if len(samples) > 0 {
		if err := model.SaveChannelRuntimeSamples(samples); err != nil {
			common.SysError("channel runtime: save samples failed: " + err.Error())
		}
	}
	if len(limits) > 0 {
		if err := model.UpsertChannelRuntimeLimits(limits); err != nil {
			common.SysError("channel runtime: save limits failed: " + err.Error())
		}
	}

	maybeCleanupChannelRuntimeSamples(setting)
}

func maybeCleanupChannelRuntimeSamples(setting *operation_setting.ChannelRuntimeSetting) {
	now := time.Now().Unix()
	last := channelRuntimeLastCleanup.Load()
	if last > 0 && now-last < int64(runtimeCleanupInterval.Seconds()) {
		return
	}
	if !channelRuntimeLastCleanup.CompareAndSwap(last, now) {
		return
	}
	cutoff := model.ChannelRuntimeRetentionCutoff(setting.SampleRetentionMonths)
	if err := model.DeleteChannelRuntimeSamplesBefore(cutoff); err != nil {
		common.SysError("channel runtime: cleanup samples failed: " + err.Error())
	}
}

// applyChannelRuntimeObservation 把渠道级聚合观测送入饱和锁状态机。
func applyChannelRuntimeObservation(
	channel *model.Channel,
	samples []model.ChannelRuntimeSample,
	limits []model.ChannelRuntimeLimit,
	collectedAt int64,
) {
	if len(samples) == 0 {
		return
	}
	observation := model.ChannelRuntimeObservation{
		ChannelId:      channel.Id,
		ChannelName:    channel.Name,
		ModelNames:     strings.Join(channel.GetModels(), ","),
		CandidateCount: model.CountRuntimeCandidateChannels(channel.Id),
		CollectedAt:    collectedAt,
	}
	for _, sample := range samples {
		observation.Running += sample.Running
		observation.Waiting += sample.Waiting
		if sample.KVCacheUsage > observation.KVCacheUsage {
			observation.KVCacheUsage = sample.KVCacheUsage
		}
		if sample.PrefixCacheHitRate > observation.PrefixCacheHitRate {
			observation.PrefixCacheHitRate = sample.PrefixCacheHitRate
		}
	}
	for _, limit := range limits {
		if limit.PeakRunning > observation.PeakRunning {
			observation.PeakRunning = limit.PeakRunning
		}
		// 最大 batch 是渠道级配置，直接取（不能按模型累加）
		observation.MaxBatch = limit.MaxBatch
		observation.MaxBatchSource = limit.Source
	}
	model.ApplyChannelRuntimeObservation(observation)
}

// collectChannelRuntime 抓取单个渠道的 metrics 并转为样本与上限记录。
func collectChannelRuntime(
	channel *model.Channel,
	setting *operation_setting.ChannelRuntimeSetting,
	collectedAt int64,
) ([]model.ChannelRuntimeSample, []model.ChannelRuntimeLimit, error) {
	metricsURL := runtimeMetricsURL(channel)
	if metricsURL == "" {
		return nil, nil, fmt.Errorf("base_url 为空，无法推导 metrics 地址")
	}
	body, err := fetchMetricsText(metricsURL, setting.ScrapeTimeoutSeconds)
	if err != nil {
		return nil, nil, err
	}
	scraped := parseVLLMMetrics(body)
	if !scraped.Found {
		return nil, nil, fmt.Errorf("%s 未包含 vllm 运行时指标", metricsURL)
	}

	channelModels := channel.GetModels()
	if len(channelModels) == 0 {
		return nil, nil, nil
	}

	// 指标带 model_name 标签时按模型对齐；没有标签（vLLM 单实例单模型）时归到渠道首个模型。
	modelNames := make([]string, 0, len(scraped.ByModel))
	if len(scraped.ByModel) > 0 {
		for modelName := range scraped.ByModel {
			modelNames = append(modelNames, modelName)
		}
	} else {
		modelNames = append(modelNames, channelModels[0])
	}

	lockedAtCollection := model.IsChannelRuntimeLocked(channel.Id)

	// 最大 batch 是渠道维度（一个渠道 = 一个 vLLM 实例 = 一个模型），
	// 优先级：按渠道名手工覆盖 > vLLM 暴露的 max_num_seqs > 全局默认（4）。
	peak := model.PeakRunningForChannel(channel.Id)
	if scraped.Total.Running > peak {
		peak = scraped.Total.Running
	}
	maxBatch, source := resolveChannelMaxBatch(setting, channel.Name, scraped.Total.MaxNumSeqs)

	now := time.Now().Unix()
	samples := make([]model.ChannelRuntimeSample, 0, len(modelNames))
	for _, modelName := range modelNames {
		series := scraped.SeriesFor(modelName)
		// 实例没有 avg_*_throughput 指标时，用累计 token 增量估算当前吞吐。
		if series.PromptThroughput == 0 && series.GenerationThroughput == 0 {
			if promptTP, generationTP, ok := deriveCounterThroughput(channel.Id, modelName, series, now); ok {
				series.PromptThroughput = promptTP
				series.GenerationThroughput = generationTP
			}
		}
		samples = append(samples, model.ChannelRuntimeSample{
			ChannelId:            channel.Id,
			ModelName:            modelName,
			CollectedAt:          collectedAt,
			Running:              series.Running,
			Waiting:              series.Waiting,
			MaxBatch:             maxBatch,
			KVCacheUsage:         series.GPUCacheUsage,
			PrefixCacheHitRate:   series.PrefixCacheHitRate,
			PromptThroughput:     series.PromptThroughput,
			GenerationThroughput: series.GenerationThroughput,
			Locked:               lockedAtCollection,
		})
	}

	limits := []model.ChannelRuntimeLimit{{
		ChannelId:   channel.Id,
		ChannelName: channel.Name,
		MaxBatch:    maxBatch,
		Source:      source,
		PeakRunning: peak,
		UpdatedAt:   collectedAt,
	}}
	return samples, limits, nil
}

// resolveChannelMaxBatch 按 渠道名手工覆盖 > /metrics 的 max_num_seqs > 全局默认(4) 解析最大并发。
// 一个渠道只服务一个模型，所以不再需要模型维度。
func resolveChannelMaxBatch(
	setting *operation_setting.ChannelRuntimeSetting,
	channelName string,
	metricMaxNumSeqs int,
) (int, string) {
	if override := setting.LookupMaxBatchOverride(channelName); override > 0 {
		return override, model.ChannelRuntimeLimitSourceManual
	}
	if metricMaxNumSeqs > 0 {
		return metricMaxNumSeqs, model.ChannelRuntimeLimitSourceMetrics
	}
	if setting.DefaultMaxBatch > 0 {
		return setting.DefaultMaxBatch, model.ChannelRuntimeLimitSourceDefault
	}
	return operation_setting.DefaultChannelMaxBatch, model.ChannelRuntimeLimitSourceDefault
}

// runtimeMetricsURL 从渠道 base_url 自动推导 metrics 地址：
// 取 scheme+host，拼上 vLLM 标准的 /metrics，忽略 base_url 里的 /v1 之类业务路径。
func runtimeMetricsURL(channel *model.Channel) string {
	baseURL := strings.TrimSpace(channel.GetBaseURL())
	if baseURL == "" {
		return ""
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	scheme := parsed.Scheme
	if scheme == "" {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s%s", scheme, parsed.Host, runtimeMetricsPath)
}

func fetchMetricsText(metricsURL string, timeoutSeconds int) (string, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = operation_setting.DefaultScrapeTimeoutSeconds
	}
	client := &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second}
	req, err := http.NewRequest(http.MethodGet, metricsURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metrics 返回状态码 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func logChannelRuntimeFailure(channelId int, err error) {
	now := time.Now().Unix()
	if last, ok := channelRuntimeLastFailureLog.Load(channelId); ok {
		if lastAt, _ := last.(int64); now-lastAt < int64(runtimeFailureLogInterval.Seconds()) {
			return
		}
	}
	channelRuntimeLastFailureLog.Store(channelId, now)
	common.SysError(fmt.Sprintf("channel runtime: collect channel #%d failed: %s", channelId, err.Error()))
}

// vllmMetricSeries 是单个模型（或无标签时的整机）的一组运行时指标。
type vllmMetricSeries struct {
	Running              int
	Waiting              int
	MaxNumSeqs           int
	GPUCacheUsage        float64
	PrefixCacheHitRate   float64
	PromptThroughput     float64
	GenerationThroughput float64
	// PromptTokens / GenerationTokens 是累计 token 数（counter），
	// 仅用于在实例没有 avg_*_throughput 指标时按增量估算吞吐。
	PromptTokens     float64
	GenerationTokens float64
}

// channelRuntimeCounter 是一次采集到的累计 token 数与采集时刻。
type channelRuntimeCounter struct {
	PromptTokens     float64
	GenerationTokens float64
	At               int64
}

// vllmMetrics 解析后的结果，ByModel 仅在指标带 model_name 标签时填充，合计值放在 Total。
type vllmMetrics struct {
	Found   bool
	Total   vllmMetricSeries
	ByModel map[string]vllmMetricSeries
}

// SeriesFor 返回指定模型的指标，无模型标签时回退到整机合计值。
func (m vllmMetrics) SeriesFor(modelName string) vllmMetricSeries {
	if series, ok := m.ByModel[modelName]; ok {
		return series
	}
	return m.Total
}

// parseVLLMMetrics 解析 Prometheus 文本格式，只提取 vLLM 运行时相关的几个指标。
func parseVLLMMetrics(text string) vllmMetrics {
	result := vllmMetrics{ByModel: make(map[string]vllmMetricSeries)}
	var prefixHits, prefixQueries float64
	hitModelHits := make(map[string]float64)
	hitModelQueries := make(map[string]float64)

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, labels, value, ok := splitMetricLine(line)
		if !ok {
			continue
		}
		modelName := labels["model_name"]
		switch name {
		case metricNumRequestsRunning:
			result.Found = true
			result.addRunning(modelName, int(value))
		case metricNumRequestsWaiting:
			result.Found = true
			result.addWaiting(modelName, int(value))
		case metricMaxNumSeqs:
			result.addMaxNumSeqs(modelName, int(value))
		case metricKVCacheUsage, metricGPUCacheUsage:
			result.addCacheUsage(modelName, value)
		case metricPrefixCacheHitRate:
			result.addPrefixHitRate(modelName, value)
		case metricPrefixCacheHits:
			if modelName == "" {
				prefixHits += value
			} else {
				hitModelHits[modelName] += value
			}
		case metricPrefixCacheQueries:
			if modelName == "" {
				prefixQueries += value
			} else {
				hitModelQueries[modelName] += value
			}
		case metricAvgPromptThroughput:
			result.addThroughput(modelName, value, true)
		case metricAvgGenerationThroughput:
			result.addThroughput(modelName, value, false)
		case metricPromptTokensTotal:
			result.addTokenCounter(modelName, value, true)
		case metricGenerationTokensTotal:
			result.addTokenCounter(modelName, value, false)
		}
	}

	if result.Total.PrefixCacheHitRate == 0 && prefixQueries > 0 {
		result.Total.PrefixCacheHitRate = prefixHits / prefixQueries * 100
	}
	for modelName, series := range result.ByModel {
		if series.PrefixCacheHitRate == 0 && hitModelQueries[modelName] > 0 {
			series.PrefixCacheHitRate = hitModelHits[modelName] / hitModelQueries[modelName] * 100
			result.ByModel[modelName] = series
		}
	}
	return result
}

func (m *vllmMetrics) addRunning(modelName string, value int) {
	m.Total.Running += value
	if modelName == "" {
		return
	}
	series := m.ByModel[modelName]
	series.Running += value
	m.ByModel[modelName] = series
}

func (m *vllmMetrics) addWaiting(modelName string, value int) {
	m.Total.Waiting += value
	if modelName == "" {
		return
	}
	series := m.ByModel[modelName]
	series.Waiting += value
	m.ByModel[modelName] = series
}

func (m *vllmMetrics) addMaxNumSeqs(modelName string, value int) {
	if value > m.Total.MaxNumSeqs {
		m.Total.MaxNumSeqs = value
	}
	if modelName == "" {
		return
	}
	series := m.ByModel[modelName]
	if value > series.MaxNumSeqs {
		series.MaxNumSeqs = value
	}
	m.ByModel[modelName] = series
}

func (m *vllmMetrics) addCacheUsage(modelName string, value float64) {
	usage := value * 100
	if usage > m.Total.GPUCacheUsage {
		m.Total.GPUCacheUsage = usage
	}
	if modelName == "" {
		return
	}
	series := m.ByModel[modelName]
	if usage > series.GPUCacheUsage {
		series.GPUCacheUsage = usage
	}
	m.ByModel[modelName] = series
}

// addThroughput 记录 prompt / generation 吞吐（tokens/s）。
// 一个实例多模型时按模型各有一行，合计值相加。
func (m *vllmMetrics) addThroughput(modelName string, value float64, prompt bool) {
	if prompt {
		m.Total.PromptThroughput += value
	} else {
		m.Total.GenerationThroughput += value
	}
	if modelName == "" {
		return
	}
	series := m.ByModel[modelName]
	if prompt {
		series.PromptThroughput = value
	} else {
		series.GenerationThroughput = value
	}
	m.ByModel[modelName] = series
}

// addTokenCounter 累加累计 token 数，一个渠道可能有多个实例/engine 行。
func (m *vllmMetrics) addTokenCounter(modelName string, value float64, prompt bool) {
	if prompt {
		m.Total.PromptTokens += value
	} else {
		m.Total.GenerationTokens += value
	}
	if modelName == "" {
		return
	}
	series := m.ByModel[modelName]
	if prompt {
		series.PromptTokens += value
	} else {
		series.GenerationTokens += value
	}
	m.ByModel[modelName] = series
}

// deriveCounterThroughput 用两次采集间累计 token 的增量估算吞吐（tokens/s）。
// 首次采集、计数器回退（实例重启）或时间间隔异常时返回 false，调用方保留原值。
func deriveCounterThroughput(channelId int, modelName string, series vllmMetricSeries, now int64) (float64, float64, bool) {
	key := fmt.Sprintf("%d/%s", channelId, modelName)
	previous, ok := channelRuntimeCounters.Load(key)
	channelRuntimeCounters.Store(key, channelRuntimeCounter{
		PromptTokens:     series.PromptTokens,
		GenerationTokens: series.GenerationTokens,
		At:               now,
	})
	if !ok {
		return 0, 0, false
	}
	prev := previous.(channelRuntimeCounter)
	elapsed := now - prev.At
	if elapsed <= 0 {
		return 0, 0, false
	}
	var prompt, generation float64
	if series.PromptTokens >= prev.PromptTokens {
		prompt = (series.PromptTokens - prev.PromptTokens) / float64(elapsed)
	}
	if series.GenerationTokens >= prev.GenerationTokens {
		generation = (series.GenerationTokens - prev.GenerationTokens) / float64(elapsed)
	}
	return prompt, generation, true
}

func (m *vllmMetrics) addPrefixHitRate(modelName string, value float64) {
	rate := value * 100
	if rate > m.Total.PrefixCacheHitRate {
		m.Total.PrefixCacheHitRate = rate
	}
	if modelName == "" {
		return
	}
	series := m.ByModel[modelName]
	if rate > series.PrefixCacheHitRate {
		series.PrefixCacheHitRate = rate
	}
	m.ByModel[modelName] = series
}

// splitMetricLine 解析 Prometheus 文本行：metric_name{label="v",...} value [timestamp]。
func splitMetricLine(line string) (string, map[string]string, float64, bool) {
	nameEnd := strings.IndexAny(line, "{ ")
	if nameEnd <= 0 {
		return "", nil, 0, false
	}
	name := line[:nameEnd]
	labels := map[string]string{}
	rest := line[nameEnd:]
	if strings.HasPrefix(rest, "{") {
		labelEnd := strings.LastIndex(rest, "}")
		if labelEnd < 0 {
			return "", nil, 0, false
		}
		labels = parseMetricLabels(rest[1:labelEnd])
		rest = rest[labelEnd+1:]
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "", nil, 0, false
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "", nil, 0, false
	}
	return name, labels, value, true
}

func parseMetricLabels(raw string) map[string]string {
	labels := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		eq := strings.Index(pair, "=")
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(pair[:eq])
		value := strings.Trim(strings.TrimSpace(pair[eq+1:]), `"`)
		labels[key] = value
	}
	return labels
}
