package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleMetricsText = `# HELP vllm:num_requests_running Number of requests in model execution batches.
# TYPE vllm:num_requests_running gauge
vllm:num_requests_running{model_name="glm-5"} 4.0
vllm:num_requests_waiting{model_name="glm-5"} 0.0
vllm:gpu_cache_usage_perc{model_name="glm-5"} 0.523
vllm:max_num_seqs{model_name="glm-5"} 32.0
vllm:prefix_cache_queries_total{model_name="glm-5"} 100.0
vllm:prefix_cache_hits_total{model_name="glm-5"} 92.0
`

const unlabeledMetricsText = `vllm:num_requests_running 5.0
vllm:num_requests_waiting 2.0
vllm:gpu_cache_usage_perc 0.581
`

func TestParseVLLMMetricsWithModelLabel(t *testing.T) {
	parsed := parseVLLMMetrics(sampleMetricsText)
	require.True(t, parsed.Found)

	series := parsed.SeriesFor("glm-5")
	assert.Equal(t, 4, series.Running)
	assert.Equal(t, 0, series.Waiting)
	assert.Equal(t, 32, series.MaxNumSeqs)
	assert.InDelta(t, 52.3, series.GPUCacheUsage, 0.001)
	assert.InDelta(t, 92, series.PrefixCacheHitRate, 0.001)
}

func TestParseVLLMMetricsWithoutLabels(t *testing.T) {
	parsed := parseVLLMMetrics(unlabeledMetricsText)
	require.True(t, parsed.Found)
	assert.Equal(t, 5, parsed.Total.Running)
	assert.Equal(t, 2, parsed.Total.Waiting)
	assert.InDelta(t, 58.1, parsed.Total.GPUCacheUsage, 0.001)

	// 无模型标签时任何模型都回退到整机合计值
	series := parsed.SeriesFor("anything")
	assert.Equal(t, 5, series.Running)
}

func TestParseVLLMMetricsIgnoresUnrelatedLines(t *testing.T) {
	parsed := parseVLLMMetrics("process_cpu_seconds_total 12.5\n# just a comment\n")
	assert.False(t, parsed.Found)
}

func TestResolveChannelMaxBatchPriority(t *testing.T) {
	setting := operation_setting.GetChannelRuntimeSetting()
	saved := *setting
	t.Cleanup(func() { *setting = saved })

	setting.DefaultMaxBatch = 8
	setting.MaxBatchOverrides = []operation_setting.ChannelMaxBatchOverride{
		{ChannelName: "glm-5-primary", MaxBatch: 64},
	}

	// 按渠道名命中手工覆盖
	maxBatch, source := resolveChannelMaxBatch(setting, "glm-5-primary", 32)
	assert.Equal(t, 64, maxBatch)
	assert.Equal(t, "manual", source)

	// 渠道名不匹配时用 metrics 暴露的 max_num_seqs
	maxBatch, source = resolveChannelMaxBatch(setting, "glm-5-secondary", 32)
	assert.Equal(t, 32, maxBatch)
	assert.Equal(t, "metrics", source)

	// metrics 没暴露时退回全局默认
	maxBatch, source = resolveChannelMaxBatch(setting, "glm-5-secondary", 0)
	assert.Equal(t, 8, maxBatch)
	assert.Equal(t, "default", source)
}

func TestResolveChannelMaxBatchFallsBackToFour(t *testing.T) {
	setting := operation_setting.GetChannelRuntimeSetting()
	saved := *setting
	t.Cleanup(func() { *setting = saved })

	setting.DefaultMaxBatch = 0
	setting.MaxBatchOverrides = nil

	// 什么都没配时默认 4
	maxBatch, source := resolveChannelMaxBatch(setting, "anything", 0)
	assert.Equal(t, operation_setting.DefaultChannelMaxBatch, maxBatch)
	assert.Equal(t, 4, maxBatch)
	assert.Equal(t, "default", source)
}

func TestRuntimeMetricsURLDerivation(t *testing.T) {
	// metrics 地址完全由渠道 base_url 推导，不再需要手工配置
	channel := &model.Channel{Id: 7, BaseURL: strPtr("http://10.210.139.118:8000/v1")}
	assert.Equal(t, "http://10.210.139.118:8000/metrics", runtimeMetricsURL(channel))

	// 业务路径不同也只看 scheme+host
	channel.BaseURL = strPtr("https://glm5.internal.example.com/api/v1/chat")
	assert.Equal(t, "https://glm5.internal.example.com/metrics", runtimeMetricsURL(channel))

	// base_url 为空时无法推导
	channel.BaseURL = nil
	assert.Equal(t, "", runtimeMetricsURL(channel))
}

func strPtr(value string) *string {
	return &value
}
