package model

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
)

// withRuntimeSetting 临时覆盖运行时监控配置，测试结束后恢复。
func withRuntimeSetting(t *testing.T, mutate func(*operation_setting.ChannelRuntimeSetting)) {
	t.Helper()
	setting := operation_setting.GetChannelRuntimeSetting()
	saved := *setting
	mutate(setting)
	operation_setting.NormalizeChannelRuntimeSetting(setting)
	ResetChannelRuntimeStates()
	t.Cleanup(func() {
		*setting = saved
		ResetChannelRuntimeStates()
	})
}

func runtimeObservation(channelId, running, waiting, maxBatch, candidates int, at int64) ChannelRuntimeObservation {
	return ChannelRuntimeObservation{
		ChannelId:      channelId,
		Running:        running,
		Waiting:        waiting,
		MaxBatch:       maxBatch,
		CandidateCount: candidates,
		CollectedAt:    at,
	}
}

func TestChannelRuntimeLocksOnSustainedQueueing(t *testing.T) {
	withRuntimeSetting(t, func(s *operation_setting.ChannelRuntimeSetting) {
		s.Enabled = true
		s.SaturationLockEnabled = true
		s.SaturationWaitingSamples = 3
		s.SaturationReleaseSamples = 1
		s.SaturationMaxLockSeconds = 300
	})

	var state ChannelRuntimeState
	for i := int64(1); i <= 3; i++ {
		state = ApplyChannelRuntimeObservation(runtimeObservation(1, 32, 4, 32, 3, i*5))
	}
	assert.True(t, state.Locked, "持续排队 3 个采样后应锁定渠道")
	assert.True(t, IsChannelRuntimeLocked(1))
}

func TestChannelRuntimeKeepsUnlockedWhenWaitingDrains(t *testing.T) {
	withRuntimeSetting(t, func(s *operation_setting.ChannelRuntimeSetting) {
		s.Enabled = true
		s.SaturationLockEnabled = true
		s.SaturationWaitingSamples = 3
	})

	// Waiting 忽高忽低说明队列在消化，不应累计到锁定
	ApplyChannelRuntimeObservation(runtimeObservation(1, 32, 8, 32, 3, 5))
	ApplyChannelRuntimeObservation(runtimeObservation(1, 32, 3, 32, 3, 10))
	ApplyChannelRuntimeObservation(runtimeObservation(1, 32, 9, 32, 3, 15))
	state := ApplyChannelRuntimeObservation(runtimeObservation(1, 32, 8, 32, 3, 20))

	assert.False(t, state.Locked, "Waiting 下降过就不应锁定")
}

func TestChannelRuntimeNeverLocksSingleCandidate(t *testing.T) {
	withRuntimeSetting(t, func(s *operation_setting.ChannelRuntimeSetting) {
		s.Enabled = true
		s.SaturationLockEnabled = true
		s.SaturationWaitingSamples = 1
	})

	state := ApplyChannelRuntimeObservation(runtimeObservation(1, 32, 8, 32, 1, 5))
	assert.False(t, state.Locked, "候选渠道只有 1 个时不能锁定")
}

func TestChannelRuntimeUnlocksWhenDrained(t *testing.T) {
	withRuntimeSetting(t, func(s *operation_setting.ChannelRuntimeSetting) {
		s.Enabled = true
		s.SaturationLockEnabled = true
		s.SaturationWaitingSamples = 1
		s.SaturationReleaseSamples = 1
		s.SaturationMaxLockSeconds = 300
	})

	locked := ApplyChannelRuntimeObservation(runtimeObservation(1, 32, 5, 32, 3, 5))
	assert.True(t, locked.Locked)

	unlocked := ApplyChannelRuntimeObservation(runtimeObservation(1, 10, 0, 32, 3, 10))
	assert.False(t, unlocked.Locked, "Running < maxBatch 且 Waiting=0 应解锁")
}

func TestChannelRuntimeUnlocksOnLockTimeout(t *testing.T) {
	withRuntimeSetting(t, func(s *operation_setting.ChannelRuntimeSetting) {
		s.Enabled = true
		s.SaturationLockEnabled = true
		s.SaturationWaitingSamples = 1
		s.SaturationReleaseSamples = 5
		s.SaturationMaxLockSeconds = 30
	})

	assert.True(t, ApplyChannelRuntimeObservation(runtimeObservation(1, 32, 5, 32, 3, 10)).Locked)

	// 仍然满载排队，但锁定已超时：必须解锁，避免采集异常导致永久冷冻
	state := ApplyChannelRuntimeObservation(runtimeObservation(1, 32, 5, 32, 3, 45))
	assert.False(t, state.Locked)
}

func TestChannelRuntimeResetOnDisable(t *testing.T) {
	withRuntimeSetting(t, func(s *operation_setting.ChannelRuntimeSetting) {
		s.Enabled = true
		s.SaturationLockEnabled = true
		s.SaturationWaitingSamples = 1
	})

	assert.True(t, ApplyChannelRuntimeObservation(runtimeObservation(1, 32, 5, 32, 3, 5)).Locked)

	setting := operation_setting.GetChannelRuntimeSetting()
	setting.Enabled = false
	state := ApplyChannelRuntimeObservation(runtimeObservation(1, 32, 5, 32, 3, 10))
	assert.False(t, state.Locked, "关闭功能后不得残留锁")
}

func TestFilterChannelIdsByRuntimeState(t *testing.T) {
	withRuntimeSetting(t, func(s *operation_setting.ChannelRuntimeSetting) {
		s.Enabled = true
		s.SaturationLockEnabled = true
		s.SaturationWaitingSamples = 1
		s.LengthRoutingFallback = true
	})

	ApplyChannelRuntimeObservation(runtimeObservation(2, 32, 5, 32, 3, 5))
	assert.True(t, IsChannelRuntimeLocked(2))

	// 长度分流限定在 1/2，锁把 2 过滤掉
	filtered := filterChannelIdsByRuntimeState([]int{1, 2}, NewChannelSelectionConstraint([]int{1, 2}, true))
	assert.Equal(t, []int{1}, filtered)

	// 只剩一个候选时不应用锁：全部被锁也得有渠道可用
	filtered = filterChannelIdsByRuntimeState([]int{2}, NewChannelSelectionConstraint([]int{2}, true))
	assert.Equal(t, []int{2}, filtered)

	// 允许集合内无候选时按 fallback 配置退回全量
	filtered = filterChannelIdsByRuntimeState([]int{1, 3}, NewChannelSelectionConstraint([]int{9}, true))
	assert.Equal(t, []int{1, 3}, filtered)

	filtered = filterChannelIdsByRuntimeState([]int{1, 3}, NewChannelSelectionConstraint([]int{9}, false))
	assert.Empty(t, filtered)
}
