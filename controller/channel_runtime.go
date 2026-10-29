package controller

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

type channelRuntimeStatusItem struct {
	ChannelId          int      `json:"channel_id"`
	ChannelName        string   `json:"channel_name"`
	ChannelStatus      int      `json:"channel_status"`
	Models             []string `json:"models"`
	MetricsURL         string   `json:"metrics_url"`
	Running            int      `json:"running"`
	Waiting            int      `json:"waiting"`
	MaxBatch           int      `json:"max_batch"`
	MaxBatchSource     string   `json:"max_batch_source"`
	PeakRunning        int      `json:"peak_running"`
	KVCacheUsage       float64  `json:"kv_cache_usage"`
	PrefixCacheHitRate float64  `json:"prefix_cache_hit_rate"`
	Locked             bool     `json:"locked"`
	LockReason         string   `json:"lock_reason"`
	LockedAt           int64    `json:"locked_at"`
	CandidateCount     int      `json:"candidate_count"`
	LastCollectedAt    int64    `json:"last_collected_at"`
	LastError          string   `json:"last_error"`
	Stale              bool     `json:"stale"`
}

// GetChannelRuntimeStatus 返回各渠道当前的 Running / Waiting / 最大 batch / 锁定状态。
func GetChannelRuntimeStatus(c *gin.Context) {
	setting := operation_setting.GetChannelRuntimeSetting()
	operation_setting.NormalizeChannelRuntimeSetting(setting)

	states := model.GetChannelRuntimeStates()
	sort.Slice(states, func(i, j int) bool {
		return states[i].ChannelId < states[j].ChannelId
	})

	// 一次采集最长可能耗时 scrape_timeout，这段时间不能算作“过期”，
	// 否则 1 秒间隔 + 3 秒超时会把正常采集误标成 stale。
	staleThreshold := int64(setting.ScrapeIntervalSeconds*3 + setting.ScrapeTimeoutSeconds)
	now := time.Now().Unix()
	items := make([]channelRuntimeStatusItem, 0, len(states))
	for _, state := range states {
		item := channelRuntimeStatusItem{
			ChannelId:          state.ChannelId,
			MetricsURL:         state.MetricsURL,
			Running:            state.Running,
			Waiting:            state.Waiting,
			MaxBatch:           state.MaxBatch,
			MaxBatchSource:     state.MaxBatchSource,
			PeakRunning:        state.PeakRunning,
			KVCacheUsage:       state.KVCacheUsage,
			PrefixCacheHitRate: state.PrefixCacheHitRate,
			Locked:             state.Locked,
			LockReason:         state.LockReason,
			LockedAt:           state.LockedAt,
			CandidateCount:     state.CandidateCount,
			LastCollectedAt:    state.LastCollectedAt,
			LastError:          state.LastError,
			Stale:              state.LastCollectedAt == 0 || now-state.LastCollectedAt > staleThreshold,
		}
		item.ChannelName = state.ChannelName
		if channel, err := model.CacheGetChannel(state.ChannelId); err == nil && channel != nil {
			item.ChannelName = channel.Name
			item.ChannelStatus = channel.Status
			item.Models = channel.GetModels()
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":                 setting.Enabled,
			"saturation_lock_enabled": setting.SaturationLockEnabled,
			"length_routing_enabled":  setting.LengthRoutingEnabled,
			"scrape_interval_seconds": setting.ScrapeIntervalSeconds,
			"items":                   items,
		},
	})
}

// GetChannelRuntimeSamples 返回时序快照，用于画 Running/Waiting 曲线。
func GetChannelRuntimeSamples(c *gin.Context) {
	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	modelName := c.Query("model_name")
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "1"))
	if hours <= 0 {
		hours = 1
	}
	if hours > 168 {
		hours = 168
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "2000"))

	endTs := time.Now().Unix()
	startTs := endTs - int64(hours)*3600
	samples, err := model.GetChannelRuntimeSamples(channelId, modelName, startTs, endTs, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": samples})
}

// channelRuntimeRange 解析看板趋势查询的时间范围与时间桶。
// 未传参数时默认最近 24 小时；时间桶按范围内最多约 400 个点自动推导。
func channelRuntimeRange(c *gin.Context) (startTs int64, endTs int64, bucketSeconds int64, ok bool) {
	endTs = time.Now().Unix()
	startTs = endTs - 86400
	if raw, err := strconv.ParseInt(c.Query("start_timestamp"), 10, 64); err == nil && raw > 0 {
		startTs = raw
	}
	if raw, err := strconv.ParseInt(c.Query("end_timestamp"), 10, 64); err == nil && raw > 0 {
		endTs = raw
	}
	if endTs <= startTs {
		return 0, 0, 0, false
	}
	// 最多回溯一年，避免一次查询扫过多历史数据
	if maxStart := endTs - 366*86400; startTs < maxStart {
		startTs = maxStart
	}
	bucketSeconds, _ = strconv.ParseInt(c.Query("bucket_seconds"), 10, 64)
	if bucketSeconds <= 0 {
		bucketSeconds = (endTs - startTs) / 400
	}
	if bucketSeconds < 10 {
		bucketSeconds = 10
	}
	if bucketSeconds > 86400 {
		bucketSeconds = 86400
	}
	return startTs, endTs, bucketSeconds, true
}

// channelRuntimeSeriesItem 是运行时趋势的一行，带上渠道名便于前端直接分组展示。
type channelRuntimeSeriesItem struct {
	ChannelId            int     `json:"channel_id"`
	ChannelName          string  `json:"channel_name"`
	ModelNames           string  `json:"model_names"`
	Bucket               int64   `json:"bucket"`
	AvgRunning           float64 `json:"avg_running"`
	MaxRunning           int     `json:"max_running"`
	AvgWaiting           float64 `json:"avg_waiting"`
	MaxWaiting           int     `json:"max_waiting"`
	MaxBatch             int     `json:"max_batch"`
	AvgRequests          float64 `json:"avg_requests"`
	MaxRequests          int     `json:"max_requests"`
	PromptThroughput     float64 `json:"prompt_throughput"`
	GenerationThroughput float64 `json:"generation_throughput"`
	SampleCount          int     `json:"sample_count"`
}

// GetChannelRuntimeSeries 返回按时间桶聚合的运行时趋势：
// Running / Waiting / 最大 batch / prefill 与 decode 吞吐，供看板画渠道趋势图。
func GetChannelRuntimeSeries(c *gin.Context) {
	startTs, endTs, bucketSeconds, ok := channelRuntimeRange(c)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的时间范围"})
		return
	}
	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	points, err := model.GetChannelRuntimeSeries(channelId, startTs, endTs, bucketSeconds)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	metas, err := model.GetChannelRuntimeMeta()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	names := make(map[int]model.ChannelMeta, len(metas))
	channels := make([]channelRuntimeChannelItem, 0, len(metas))
	for _, meta := range metas {
		names[meta.Id] = meta
		channels = append(channels, channelRuntimeChannelItem{
			Id:     meta.Id,
			Name:   meta.Name,
			Models: splitChannelModels(meta.Models),
			Status: meta.Status,
		})
	}
	items := make([]channelRuntimeSeriesItem, 0, len(points))
	for _, point := range points {
		item := channelRuntimeSeriesItem{
			ChannelId:            point.ChannelId,
			Bucket:               point.Bucket,
			AvgRunning:           point.AvgRunning,
			MaxRunning:           point.MaxRunning,
			AvgWaiting:           point.AvgWaiting,
			MaxWaiting:           point.MaxWaiting,
			MaxBatch:             point.MaxBatch,
			AvgRequests:          point.AvgRequests,
			MaxRequests:          point.MaxRequests,
			PromptThroughput:     point.PromptThroughput,
			GenerationThroughput: point.GenerationThroughput,
			SampleCount:          point.SampleCount,
		}
		if meta, found := names[point.ChannelId]; found {
			item.ChannelName = meta.Name
			item.ModelNames = meta.Models
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"start_timestamp": startTs,
			"end_timestamp":   endTs,
			"bucket_seconds":  bucketSeconds,
			"items":           items,
			"channels":        channels,
		},
	})
}

// channelUsageSeriesItem 是渠道调用趋势的一行（带渠道名，前端无需二次关联）。
type channelUsageSeriesItem struct {
	ChannelId        int    `json:"channel_id"`
	ChannelName      string `json:"channel_name"`
	ModelNames       string `json:"model_names"`
	Bucket           int64  `json:"bucket"`
	Requests         int64  `json:"requests"`
	PromptTokens     int64  `json:"prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens"`
}

// GetChannelUsageSeries 返回按渠道 + 时间桶聚合的调用量，供看板「渠道调用分析」。
func GetChannelUsageSeries(c *gin.Context) {
	startTs, endTs, bucketSeconds, ok := channelRuntimeRange(c)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的时间范围"})
		return
	}
	points, err := model.SumChannelUsageSeries(startTs, endTs, bucketSeconds)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	metas, err := model.GetChannelRuntimeMeta()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	names := make(map[int]model.ChannelMeta, len(metas))
	for _, meta := range metas {
		names[meta.Id] = meta
	}
	items := make([]channelUsageSeriesItem, 0, len(points))
	for _, point := range points {
		item := channelUsageSeriesItem{
			ChannelId:        point.ChannelId,
			Bucket:           point.Bucket,
			Requests:         point.Requests,
			PromptTokens:     point.PromptTokens,
			CompletionTokens: point.CompletionTokens,
		}
		if meta, found := names[point.ChannelId]; found {
			item.ChannelName = meta.Name
			item.ModelNames = meta.Models
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"start_timestamp": startTs,
			"end_timestamp":   endTs,
			"bucket_seconds":  bucketSeconds,
			"items":           items,
		},
	})
}

// GetChannelRuntimeLimits 返回渠道维度的最大 batch 解析结果。
func GetChannelRuntimeLimits(c *gin.Context) {
	limits, err := model.GetChannelRuntimeLimits()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": limits})
}

// channelRuntimeChannelItem 是配置页用来「默认列出全部渠道」的轻量条目。
type channelRuntimeChannelItem struct {
	Id     int      `json:"id"`
	Name   string   `json:"name"`
	Models []string `json:"models"`
	Status int      `json:"status"`
}

// splitChannelModels 把渠道 models 字段（逗号分隔）拆成列表，空值返回空数组。
func splitChannelModels(raw string) []string {
	names := make([]string, 0)
	for _, name := range strings.Split(raw, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// GetChannelRuntimeChannels 返回环境中的全部渠道（不含密钥），
// 供配置页把最大 batch / 长度分离的默认行预填成真实渠道。
func GetChannelRuntimeChannels(c *gin.Context) {
	metas, err := model.GetChannelRuntimeMeta()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	items := make([]channelRuntimeChannelItem, 0, len(metas))
	for _, meta := range metas {
		items = append(items, channelRuntimeChannelItem{
			Id:     meta.Id,
			Name:   meta.Name,
			Models: splitChannelModels(meta.Models),
			Status: meta.Status,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

// RefreshChannelRuntime 立即触发一次采集。
func RefreshChannelRuntime(c *gin.Context) {
	service.TriggerChannelRuntimeCollection()
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "采集已触发"})
}

// UnlockChannelRuntime 手工解除某渠道的饱和锁。
func UnlockChannelRuntime(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil || channelId <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的渠道 id"})
		return
	}
	if !model.ForceUnlockChannel(channelId) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "该渠道当前未被锁定"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "已解锁"})
}

// channelResourceItem 是看板「渠道资源分析」的一行：调用情况 + 当前拥挤程度。
type channelResourceItem struct {
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	ModelNames  string `json:"model_names"`
	Status      int    `json:"status"`

	// 调用情况（时间窗内，来自 logs）
	Requests         int64   `json:"requests"`
	ErrorRequests    int64   `json:"error_requests"`
	ErrorRate        float64 `json:"error_rate"`
	Quota            int64   `json:"quota"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	AvgUseTimeMs     float64 `json:"avg_use_time_ms"`

	// 拥挤程度（当前内存态，来自 vLLM /metrics）
	Running            int     `json:"running"`
	Waiting            int     `json:"waiting"`
	MaxBatch           int     `json:"max_batch"`
	MaxBatchSource     string  `json:"max_batch_source"`
	Utilization        float64 `json:"utilization"`
	KVCacheUsage       float64 `json:"kv_cache_usage"`
	PrefixCacheHitRate float64 `json:"prefix_cache_hit_rate"`
	Locked             bool    `json:"locked"`
	HasRuntimeData     bool    `json:"has_runtime_data"`
	LastCollectedAt    int64   `json:"last_collected_at"`
}

// GetChannelResourceStats 返回看板「渠道资源分析」所需的按渠道聚合数据：
// 调用情况（logs 聚合）+ 当前 Running/Waiting/最大 batch（运行时内存态）。
func GetChannelResourceStats(c *gin.Context) {
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	if hours <= 0 {
		hours = 24
	}
	if hours > 168 {
		hours = 168
	}

	setting := operation_setting.GetChannelRuntimeSetting()
	operation_setting.NormalizeChannelRuntimeSetting(setting)

	endTs := time.Now().Unix()
	startTs := endTs - int64(hours)*3600

	metas, err := model.GetChannelRuntimeMeta()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	usage, err := model.SumChannelUsageByWindow(startTs, endTs)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	states := make(map[int]model.ChannelRuntimeState)
	for _, state := range model.GetChannelRuntimeStates() {
		states[state.ChannelId] = state
	}

	items := make([]channelResourceItem, 0, len(metas))
	for _, meta := range metas {
		item := channelResourceItem{
			ChannelId:   meta.Id,
			ChannelName: meta.Name,
			ModelNames:  meta.Models,
			Status:      meta.Status,
		}
		if summary, ok := usage[meta.Id]; ok {
			item.Requests = summary.Requests
			item.ErrorRequests = summary.ErrorRequests
			item.PromptTokens = summary.PromptTokens
			item.CompletionTokens = summary.CompletionTokens
			item.TotalTokens = summary.PromptTokens + summary.CompletionTokens
			item.AvgUseTimeMs = summary.AvgUseTimeMs
		}
		total := item.Requests + item.ErrorRequests
		if total > 0 {
			item.ErrorRate = float64(item.ErrorRequests) / float64(total) * 100
		}

		if state, ok := states[meta.Id]; ok {
			item.HasRuntimeData = state.LastCollectedAt > 0
			item.Running = state.Running
			item.Waiting = state.Waiting
			item.MaxBatch = state.MaxBatch
			item.MaxBatchSource = state.MaxBatchSource
			item.KVCacheUsage = state.KVCacheUsage
			item.PrefixCacheHitRate = state.PrefixCacheHitRate
			item.Locked = state.Locked
			item.LastCollectedAt = state.LastCollectedAt
			if state.ChannelName != "" {
				item.ChannelName = state.ChannelName
			}
			if state.ModelNames != "" {
				item.ModelNames = state.ModelNames
			}
		}
		if item.MaxBatch > 0 {
			item.Utilization = float64(item.Running) / float64(item.MaxBatch) * 100
		}

		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Requests != items[j].Requests {
			return items[i].Requests > items[j].Requests
		}
		return items[i].ChannelId < items[j].ChannelId
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"hours":                   hours,
			"enabled":                 setting.Enabled,
			"saturation_lock_enabled": setting.SaturationLockEnabled,
			"scrape_interval_seconds": setting.ScrapeIntervalSeconds,
			"items":                   items,
		},
	})
}
