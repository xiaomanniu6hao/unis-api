package model

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"gorm.io/gorm/clause"
)

// ChannelRuntimeSample 是每次采集得到的一条「渠道 × 模型」运行时快照（时序数据）。
// collected_at 按采集间隔对齐，配合唯一索引可在多节点部署时天然去重。
type ChannelRuntimeSample struct {
	Id                 int     `json:"id" gorm:"primaryKey"`
	ChannelId          int     `json:"channel_id" gorm:"uniqueIndex:idx_channel_runtime_sample_unique,priority:1;index:idx_channel_runtime_sample_ts,priority:1"`
	ModelName          string  `json:"model_name" gorm:"size:128;uniqueIndex:idx_channel_runtime_sample_unique,priority:2"`
	CollectedAt        int64   `json:"collected_at" gorm:"uniqueIndex:idx_channel_runtime_sample_unique,priority:3;index:idx_channel_runtime_sample_ts,priority:2"`
	Running            int     `json:"running"`
	Waiting            int     `json:"waiting"`
	MaxBatch           int     `json:"max_batch"`
	KVCacheUsage       float64 `json:"kv_cache_usage"`
	PrefixCacheHitRate float64 `json:"prefix_cache_hit_rate"`
	Locked             bool    `json:"locked"`
}

func (ChannelRuntimeSample) TableName() string {
	return "channel_runtime_samples"
}

// ChannelRuntimeLimit 保存每个渠道解析后的最大 batch 并发数与历史峰值并发。
// 一个渠道对应一个 vLLM 实例（单模型），因此主键只有 channel_id，不再绑定模型名。
// Source 说明取值来源：manual（后台按渠道名覆盖）/ metrics（vLLM 暴露的 max_num_seqs）/ default（全局默认）。
type ChannelRuntimeLimit struct {
	ChannelId   int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false"`
	ChannelName string `json:"channel_name" gorm:"size:128;index"`
	MaxBatch    int    `json:"max_batch"`
	Source      string `json:"source" gorm:"size:16"`
	PeakRunning int    `json:"peak_running"`
	UpdatedAt   int64  `json:"updated_at"`
}

func (ChannelRuntimeLimit) TableName() string {
	return "channel_runtime_limits"
}

const (
	ChannelRuntimeLimitSourceManual   = "manual"
	ChannelRuntimeLimitSourceMetrics  = "metrics"
	ChannelRuntimeLimitSourceObserved = "observed"
	ChannelRuntimeLimitSourceDefault  = "default"
)

// ChannelRuntimeState 是单个渠道当前的内存态，供调度过滤与后台展示使用。
type ChannelRuntimeState struct {
	ChannelId          int     `json:"channel_id"`
	ChannelName        string  `json:"channel_name"`
	ModelNames         string  `json:"model_names"`
	Running            int     `json:"running"`
	Waiting            int     `json:"waiting"`
	MaxBatch           int     `json:"max_batch"`
	MaxBatchSource     string  `json:"max_batch_source"`
	PeakRunning        int     `json:"peak_running"`
	KVCacheUsage       float64 `json:"kv_cache_usage"`
	PrefixCacheHitRate float64 `json:"prefix_cache_hit_rate"`
	Locked             bool    `json:"locked"`
	LockReason         string  `json:"lock_reason"`
	LockedAt           int64   `json:"locked_at"`
	PressureStreak     int     `json:"pressure_streak"`
	ReleaseStreak      int     `json:"release_streak"`
	CandidateCount     int     `json:"candidate_count"`
	LastCollectedAt    int64   `json:"last_collected_at"`
	LastError          string  `json:"last_error"`
	LastErrorAt        int64   `json:"last_error_at"`
	MetricsURL         string  `json:"metrics_url"`
}

// ChannelRuntimeObservation 是采集器提交给状态机的一次渠道级观测（同渠道多模型已聚合成一条）。
// MaxBatch 为各模型上限之和，Waiting/ Running 同理；CandidateCount 为该渠道任一模型在可用分组内的候选渠道数。
type ChannelRuntimeObservation struct {
	ChannelId          int
	ChannelName        string
	ModelNames         string
	Running            int
	Waiting            int
	MaxBatch           int
	MaxBatchSource     string
	PeakRunning        int
	KVCacheUsage       float64
	PrefixCacheHitRate float64
	CandidateCount     int
	MetricsURL         string
	CollectedAt        int64
}

var (
	channelRuntimeLock  sync.RWMutex
	channelRuntimeState = make(map[int]*ChannelRuntimeState)
)

// ApplyChannelRuntimeObservation 把一次观测送入饱和锁状态机，返回更新后的渠道状态。
//
// 锁定条件（全部满足）：
//  1. Running >= MaxBatch（MaxBatch 未知时只用默认值兜底，为 0 则不判定）
//  2. Waiting 连续 N 次采样大于 0 且不下降（N = saturation_waiting_samples）
//  3. 该渠道某模型的候选渠道数 > 1（不能把最后一个可用渠道锁掉）
//
// 解锁条件：Running < MaxBatch 且 Waiting == 0（连续 M 次采样，M = saturation_release_samples），
// 或者锁定时长超过 saturation_max_lock_seconds（默认 300s，防止采集异常导致渠道被永久冷冻）。
func ApplyChannelRuntimeObservation(obs ChannelRuntimeObservation) ChannelRuntimeState {
	setting := operation_setting.GetChannelRuntimeSetting()
	operation_setting.NormalizeChannelRuntimeSetting(setting)

	channelRuntimeLock.Lock()
	defer channelRuntimeLock.Unlock()

	state, ok := channelRuntimeState[obs.ChannelId]
	if !ok {
		state = &ChannelRuntimeState{ChannelId: obs.ChannelId}
		channelRuntimeState[obs.ChannelId] = state
	}
	state.ChannelName = obs.ChannelName
	state.ModelNames = obs.ModelNames

	previousWaiting := state.Waiting
	state.Running = obs.Running
	state.Waiting = obs.Waiting
	state.MaxBatch = obs.MaxBatch
	state.MaxBatchSource = obs.MaxBatchSource
	if obs.PeakRunning > state.PeakRunning {
		state.PeakRunning = obs.PeakRunning
	}
	state.KVCacheUsage = obs.KVCacheUsage
	state.PrefixCacheHitRate = obs.PrefixCacheHitRate
	state.CandidateCount = obs.CandidateCount
	state.MetricsURL = obs.MetricsURL
	state.LastCollectedAt = obs.CollectedAt
	state.LastError = ""
	state.LastErrorAt = 0

	if !setting.Enabled || !setting.SaturationLockEnabled {
		// 功能关闭时不允许残留锁，否则选择层会一直跳过该渠道
		state.Locked = false
		state.LockReason = ""
		state.LockedAt = 0
		state.PressureStreak = 0
		state.ReleaseStreak = 0
		return *state
	}

	maxBatch := obs.MaxBatch
	if maxBatch <= 0 || obs.CandidateCount <= 1 {
		state.PressureStreak = 0
	}

	if state.Locked {
		if setting.SaturationMaxLockSeconds > 0 && state.LockedAt > 0 &&
			obs.CollectedAt-state.LockedAt >= int64(setting.SaturationMaxLockSeconds) {
			releaseChannelRuntimeLock(state, "lock timeout")
			return *state
		}
		if maxBatch > 0 && obs.Running < maxBatch && obs.Waiting == 0 {
			state.ReleaseStreak++
			if state.ReleaseStreak >= setting.SaturationReleaseSamples {
				releaseChannelRuntimeLock(state, "recovered")
			}
		} else {
			state.ReleaseStreak = 0
		}
		return *state
	}

	// Waiting 持续大于 0 且不下降：出现排队说明引擎吞吐已打满。
	// 只要本次 Waiting 比上次少（队列在消化）或者 Running 掉到 batch 之下，计数清零重新累计。
	if maxBatch > 0 && obs.CandidateCount > 1 && obs.Running >= maxBatch &&
		obs.Waiting > 0 && obs.Waiting >= previousWaiting {
		state.PressureStreak++
	} else {
		state.PressureStreak = 0
	}

	if maxBatch > 0 && state.PressureStreak >= setting.SaturationWaitingSamples {
		state.Locked = true
		state.LockedAt = obs.CollectedAt
		state.LockReason = fmt.Sprintf(
			"saturated: running=%d max_batch=%d waiting=%d candidates=%d streak=%d",
			obs.Running, maxBatch, obs.Waiting, obs.CandidateCount, state.PressureStreak,
		)
		state.PressureStreak = 0
		state.ReleaseStreak = 0
		common.SysLog(fmt.Sprintf("channel #%d runtime locked: %s", obs.ChannelId, state.LockReason))
	}
	return *state
}

func releaseChannelRuntimeLock(state *ChannelRuntimeState, reason string) {
	state.Locked = false
	state.LockedAt = 0
	state.LockReason = ""
	state.PressureStreak = 0
	state.ReleaseStreak = 0
	common.SysLog(fmt.Sprintf("channel #%d runtime unlocked: %s", state.ChannelId, reason))
}

// RecordChannelRuntimeError 记录一次采集失败。连续失败到阈值时解除锁，避免采集侧故障把渠道永久冷冻。
func RecordChannelRuntimeError(channelId int, metricsURL string, err error, collectedAt int64) {
	if err == nil {
		return
	}
	setting := operation_setting.GetChannelRuntimeSetting()
	operation_setting.NormalizeChannelRuntimeSetting(setting)
	// 允许连续失败约 6 次采集间隔后自动解锁
	failureGrace := int64(setting.ScrapeIntervalSeconds * 6)

	channelRuntimeLock.Lock()
	defer channelRuntimeLock.Unlock()

	state, ok := channelRuntimeState[channelId]
	if !ok {
		state = &ChannelRuntimeState{ChannelId: channelId}
		channelRuntimeState[channelId] = state
	}
	state.MetricsURL = metricsURL
	state.LastError = err.Error()
	state.LastErrorAt = collectedAt
	if state.Locked && state.LastErrorAt-state.LockedAt >= failureGrace {
		releaseChannelRuntimeLock(state, "metrics unavailable")
	}
}

// ForceUnlockChannel 手工解锁（后台运维接口）。
func ForceUnlockChannel(channelId int) bool {
	channelRuntimeLock.Lock()
	defer channelRuntimeLock.Unlock()
	state, ok := channelRuntimeState[channelId]
	if !ok || !state.Locked {
		return false
	}
	releaseChannelRuntimeLock(state, "manual")
	return true
}

// ResetChannelRuntimeStates 清空全部内存态（关闭功能或重置时调用）。
func ResetChannelRuntimeStates() {
	channelRuntimeLock.Lock()
	defer channelRuntimeLock.Unlock()
	channelRuntimeState = make(map[int]*ChannelRuntimeState)
}

// ClearChannelRuntimeState 丢弃单个渠道的内存态：渠道被移出采集范围后，
// 残留状态（尤其是饱和锁）必须一起清掉，否则会一直跳过该渠道。
func ClearChannelRuntimeState(channelId int) {
	channelRuntimeLock.Lock()
	defer channelRuntimeLock.Unlock()
	delete(channelRuntimeState, channelId)
}

// GetChannelRuntimeStates 返回全部渠道当前运行时状态的快照。
func GetChannelRuntimeStates() []ChannelRuntimeState {
	channelRuntimeLock.RLock()
	defer channelRuntimeLock.RUnlock()
	states := make([]ChannelRuntimeState, 0, len(channelRuntimeState))
	for _, state := range channelRuntimeState {
		states = append(states, *state)
	}
	return states
}

// GetChannelRuntimeState 返回单个渠道的运行时状态。
func GetChannelRuntimeState(channelId int) (ChannelRuntimeState, bool) {
	channelRuntimeLock.RLock()
	defer channelRuntimeLock.RUnlock()
	state, ok := channelRuntimeState[channelId]
	if !ok {
		return ChannelRuntimeState{}, false
	}
	return *state, true
}

// IsChannelRuntimeLocked 供选择层判断渠道是否被饱和锁冷冻。
func IsChannelRuntimeLocked(channelId int) bool {
	channelRuntimeLock.RLock()
	defer channelRuntimeLock.RUnlock()
	state, ok := channelRuntimeState[channelId]
	return ok && state.Locked
}

// ChannelSelectionConstraint 是请求级的候选渠道约束（目前用于按输入长度分流）。
type ChannelSelectionConstraint struct {
	// AllowedChannelIds 非 nil 时只允许这些渠道；nil 表示不限制。
	AllowedChannelIds map[int]struct{}
	// FallbackToAll 表示允许集合内无可用渠道时退回全量候选，避免配置写错直接 503。
	FallbackToAll bool
}

// NewChannelSelectionConstraint 由渠道 id 列表构造约束，列表为空时返回 nil（等同不限制）。
func NewChannelSelectionConstraint(channelIds []int, fallbackToAll bool) *ChannelSelectionConstraint {
	if len(channelIds) == 0 {
		return nil
	}
	allowed := make(map[int]struct{}, len(channelIds))
	for _, id := range channelIds {
		if id > 0 {
			allowed[id] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	return &ChannelSelectionConstraint{AllowedChannelIds: allowed, FallbackToAll: fallbackToAll}
}

// filterChannelIdsByRuntimeState 按长度分流 + 饱和锁过滤候选渠道。
// 关键保护：任何过滤都不得把候选清空——
//   - 长度分流命中但集合内无可用渠道时，按配置退回全量候选；
//   - 饱和锁最多跳过到剩 1 个候选为止。
func filterChannelIdsByRuntimeState(channelIds []int, constraint *ChannelSelectionConstraint) []int {
	if len(channelIds) == 0 {
		return channelIds
	}
	setting := operation_setting.GetChannelRuntimeSetting()
	operation_setting.NormalizeChannelRuntimeSetting(setting)

	result := channelIds
	if constraint != nil && constraint.AllowedChannelIds != nil {
		allowed := make([]int, 0, len(channelIds))
		for _, id := range channelIds {
			if _, ok := constraint.AllowedChannelIds[id]; ok {
				allowed = append(allowed, id)
			}
		}
		if len(allowed) == 0 {
			if !constraint.FallbackToAll {
				return nil
			}
		} else {
			result = allowed
		}
	}

	if !setting.Enabled || !setting.SaturationLockEnabled || len(result) <= 1 {
		return result
	}

	unlocked := make([]int, 0, len(result))
	lockedIds := make([]int, 0)
	for _, id := range result {
		if IsChannelRuntimeLocked(id) {
			lockedIds = append(lockedIds, id)
			continue
		}
		unlocked = append(unlocked, id)
	}
	if len(unlocked) == 0 {
		// 全部被锁时不做过滤，宁可过载也不能无渠道可用
		return result
	}
	if len(lockedIds) > 0 {
		common.SysLog(fmt.Sprintf("channel runtime: skip saturated channels %v", lockedIds))
	}
	return unlocked
}

// countRuntimeCandidateChannels 统计该渠道所服务模型的最大候选渠道数（同分组内）。
// 返回值 > 1 表示还有别的渠道可以接流量，才允许锁定。
func countRuntimeCandidateChannels(channel *Channel) int {
	if channel == nil {
		return 0
	}
	if !common.MemoryCacheEnabled {
		return countRuntimeCandidateChannelsFromDB(channel)
	}
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()
	if group2model2channels == nil {
		return 0
	}
	maxCount := 0
	for _, group := range channel.GetGroups() {
		modelMap := group2model2channels[group]
		if modelMap == nil {
			continue
		}
		for _, modelName := range channel.GetModels() {
			if count := len(modelMap[modelName]); count > maxCount {
				maxCount = count
			}
		}
	}
	return maxCount
}

func countRuntimeCandidateChannelsFromDB(channel *Channel) int {
	maxCount := 0
	for _, group := range channel.GetGroups() {
		for _, modelName := range channel.GetModels() {
			var count int64
			if err := DB.Model(&Ability{}).
				Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, modelName, true).
				Count(&count).Error; err != nil {
				continue
			}
			if int(count) > maxCount {
				maxCount = int(count)
			}
		}
	}
	return maxCount
}

// CountRuntimeCandidateChannels 暴露给采集器的候选渠道数计算。
func CountRuntimeCandidateChannels(channelId int) int {
	channel, err := CacheGetChannel(channelId)
	if err != nil || channel == nil {
		return 0
	}
	return countRuntimeCandidateChannels(channel)
}

// GetEnabledChannelIds 返回所有启用渠道的 id，供采集器每轮遍历（只查 id，代价很低）。
func GetEnabledChannelIds() ([]int, error) {
	var ids []int
	err := DB.Model(&Channel{}).Where("status = ?", common.ChannelStatusEnabled).Order("id ASC").Pluck("id", &ids).Error
	return ids, err
}

// SaveChannelRuntimeSamples 批量写入运行时快照，重复时间桶直接忽略（多节点部署去重）。
func SaveChannelRuntimeSamples(samples []ChannelRuntimeSample) error {
	if len(samples) == 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(samples, 100).Error
}

// UpsertChannelRuntimeLimits 批量保存渠道维度的最大 batch 解析结果。
func UpsertChannelRuntimeLimits(limits []ChannelRuntimeLimit) error {
	if len(limits) == 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "channel_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"channel_name", "max_batch", "source", "peak_running", "updated_at"}),
	}).CreateInBatches(limits, 100).Error
}

// GetChannelRuntimeLimits 读取全部最大 batch 记录。
func GetChannelRuntimeLimits() ([]ChannelRuntimeLimit, error) {
	var limits []ChannelRuntimeLimit
	err := DB.Order("channel_id ASC").Find(&limits).Error
	return limits, err
}

// PeakRunningForChannel 返回渠道已记录的历史峰值并发。
func PeakRunningForChannel(channelId int) int {
	var limits []ChannelRuntimeLimit
	err := DB.Select("peak_running").
		Where("channel_id = ?", channelId).
		Limit(1).
		Find(&limits).Error
	if err != nil || len(limits) == 0 {
		return 0
	}
	return limits[0].PeakRunning
}

// GetChannelRuntimeSamples 读取某渠道（可选模型）在时间窗内的快照。
func GetChannelRuntimeSamples(channelId int, modelName string, startTs int64, endTs int64, limit int) ([]ChannelRuntimeSample, error) {
	var samples []ChannelRuntimeSample
	query := DB.Model(&ChannelRuntimeSample{}).Where("collected_at >= ? AND collected_at <= ?", startTs, endTs)
	if channelId > 0 {
		query = query.Where("channel_id = ?", channelId)
	}
	if modelName != "" {
		query = query.Where("model_name = ?", modelName)
	}
	if limit <= 0 || limit > 10000 {
		limit = 2000
	}
	err := query.Order("collected_at DESC").Limit(limit).Find(&samples).Error
	return samples, err
}

// DeleteChannelRuntimeSamplesBefore 清理过期的时序数据。
func DeleteChannelRuntimeSamplesBefore(cutoffTs int64) error {
	if cutoffTs <= 0 {
		return nil
	}
	return DB.Where("collected_at < ?", cutoffTs).Delete(&ChannelRuntimeSample{}).Error
}

// ChannelRuntimeRetentionCutoff 计算保留期截止时间戳（按月，1 个月 = 一个自然月）。
func ChannelRuntimeRetentionCutoff(months int) int64 {
	if months <= 0 {
		months = operation_setting.DefaultSampleRetentionMonths
	}
	return time.Now().AddDate(0, -months, 0).Unix()
}

// ChannelMeta 是渠道的轻量元信息，供运行时/资源分析展示使用（不含密钥）。
type ChannelMeta struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Models  string `json:"models"`
	Status  int    `json:"status"`
	BaseURL string `json:"base_url"`
}

// GetChannelRuntimeMeta 返回全部渠道的轻量元信息（id/名称/模型/状态/base_url）。
func GetChannelRuntimeMeta() ([]ChannelMeta, error) {
	var metas []ChannelMeta
	err := DB.Model(&Channel{}).
		Select("id, name, models, status, base_url").
		Order("id ASC").
		Find(&metas).Error
	return metas, err
}

// GetChannelIdByName 按渠道名精确查找渠道 id（长度分流按渠道名配置，调度需要 id）。
// 未找到时返回 (0, nil)，由调用方按「不存在」处理。
func GetChannelIdByName(name string) (int, error) {
	var channel Channel
	err := DB.Select("id").Where("name = ?", name).Limit(1).Find(&channel).Error
	if err != nil {
		return 0, err
	}
	return channel.Id, nil
}

// ChannelUsageSummary 是某渠道在时间窗内的调用汇总（来自 logs 表）。
type ChannelUsageSummary struct {
	ChannelId        int     `json:"channel_id"`
	Requests         int64   `json:"requests"`
	ErrorRequests    int64   `json:"error_requests"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	AvgUseTimeMs     float64 `json:"avg_use_time_ms"`
}

// SumChannelUsageByWindow 按渠道聚合时间窗内的调用情况（成功计费日志 + 错误日志）。
func SumChannelUsageByWindow(startTs int64, endTs int64) (map[int]ChannelUsageSummary, error) {
	summaries := make(map[int]ChannelUsageSummary)

	var consumeRows []ChannelUsageSummary
	err := LOG_DB.Table("logs").
		Select("channel_id, count(*) AS requests, "+
			"COALESCE(sum(prompt_tokens), 0) AS prompt_tokens, "+
			"COALESCE(sum(completion_tokens), 0) AS completion_tokens, "+
			"COALESCE(avg(use_time), 0) * 1000 AS avg_use_time_ms").
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, startTs, endTs).
		Group("channel_id").
		Scan(&consumeRows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range consumeRows {
		summaries[row.ChannelId] = row
	}

	var errorRows []ChannelUsageSummary
	err = LOG_DB.Table("logs").
		Select("channel_id, count(*) AS error_requests").
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeError, startTs, endTs).
		Group("channel_id").
		Scan(&errorRows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range errorRows {
		summary := summaries[row.ChannelId]
		summary.ChannelId = row.ChannelId
		summary.ErrorRequests = row.ErrorRequests
		summaries[row.ChannelId] = summary
	}

	return summaries, nil
}
