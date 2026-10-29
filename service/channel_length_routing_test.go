package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/tokenkit"

	"github.com/stretchr/testify/assert"
)

// 中文请求的真实 token 数必须按 BPE 计：Estimate 的字符权重会把 2400 token
// 算成 4100，让 [0,4096) 的短请求误命中长上下文规则。
func TestCountInputTokensUsesBPEProxyForUnknownFamily(t *testing.T) {
	cjk := strings.Repeat("测试文本内容", 800) // 4800 汉字，vLLM 实测 2408 prompt tokens
	body := []byte(fmt.Sprintf(
		`{"model":"qwen3_8b","messages":[{"role":"user","content":"%s"}],"max_tokens":1}`,
		cjk,
	))

	got := countInputTokens("qwen3_8b", body)
	assert.InDelta(t, 2408, got, 120, "应贴近 vLLM 实测的 2408 token")
	assert.Less(t, got, 4096, "2408 token 的请求不能跨过 4096 边界")

	// 旧实现（Estimate）正是这个 bug 的来源，留在断言里防止回退
	assert.GreaterOrEqual(t, tokenkit.Estimate("qwen3_8b", string(body)), 4096)
}

// 英文请求按 cl100k 计：o200k 会把 4507 token 的英文算成 4000，跌破 4096 边界。
func TestCountInputTokensUsesCL100kForLatinText(t *testing.T) {
	// 与联调脚本一致：22500 字符英文，vLLM 实测 4507 prompt tokens
	en := strings.Repeat("hello world testing the quick brown fox jumps", 3000)[:22500]
	assert.InDelta(t, 4507, countInputTokens("qwen3_8b", []byte(en)), 120)

	body := []byte(fmt.Sprintf(
		`{"model":"qwen3_8b","messages":[{"role":"user","content":"%s"}],"max_tokens":1}`,
		strings.Repeat("hello world testing the quick brown fox jumps ", 300),
	))
	// 请求体小于精确计数上限，走不采样、不外推的路径
	assert.Less(t, len(body), lengthRouteExactCountBytes)
	assert.Equal(t, tokenkit.Count("gpt-4", string(body)), countInputTokens("qwen3_8b", body))
}

// OpenAI 家族仍按自己的 tiktoken 编码计数，不走代理编码。
func TestCountInputTokensKeepsOpenAIEncoding(t *testing.T) {
	body := []byte(fmt.Sprintf(
		`{"model":"gpt-4o","messages":[{"role":"user","content":"%s"}],"max_tokens":1}`,
		strings.Repeat("hello world testing ", 400),
	))
	assert.Equal(t, tokenkit.Count("gpt-4o", string(body)), countInputTokens("gpt-4o", body))
}

// 超过精确计数上限的请求体按前 16KB 采样后外推，误差要足够小。
func TestCountInputTokensExtrapolatesLargeBody(t *testing.T) {
	body := []byte(strings.Repeat(`测试文本内容，hello world. `, 3000)) // ≈ 240KB
	exact := tokenkit.Count(lengthRouteCJKModel, string(body))
	got := countInputTokens("qwen3_8b", body)

	assert.Greater(t, len(body), lengthRouteExactCountBytes)
	assert.InDelta(t, exact, got, float64(exact)*0.05, "采样外推误差应小于 5%%")
}

// 采样截断点落在多字节字符中间时不能把半个字符算成一个 token。
func TestCountInputTokensHandlesSplitRuneAtSampleBoundary(t *testing.T) {
	// 16KB 不是 3 的整数倍，这里必然切在某个汉字的中间
	body := []byte(strings.Repeat("测", 20000))
	assert.NotEqual(t, 0, lengthRouteExactCountBytes%3)

	got := countInputTokens("qwen3_8b", body)
	exact := tokenkit.Count(lengthRouteCJKModel, string(body))

	assert.Greater(t, got, 0)
	assert.InDelta(t, exact, got, float64(exact)*0.05)
}

func TestMatchLengthRoutePrefersMostSpecificRange(t *testing.T) {
	setting := &operation_setting.ChannelRuntimeSetting{
		LengthRoutingRules: []operation_setting.ChannelLengthRoute{
			{ModelName: "glm-5", ChannelName: "glm-disabled", MinLength: 0, MaxLength: 0, Enabled: false},
			{ModelName: "glm-5", ChannelName: "glm-short", MinLength: 0, MaxLength: 0, Enabled: true},
			{ModelName: "glm-5", ChannelName: "glm-long", MinLength: 10000, MaxLength: 0, Enabled: true},
			{ModelName: "glm-5", ChannelName: "glm-mid", MinLength: 1000, MaxLength: 2000, Enabled: true},
		},
	}
	operation_setting.NormalizeChannelRuntimeSetting(setting)

	// 没有更具体的区间命中时走 [0,0] 兜底
	assert.Equal(t, "glm-short", matchLengthRoute(setting, "glm-5", 0).ChannelName)
	assert.Equal(t, "glm-short", matchLengthRoute(setting, "glm-5", 999).ChannelName)
	// 落在 [1000,2000) 内取下界更大的规则
	assert.Equal(t, "glm-mid", matchLengthRoute(setting, "glm-5", 1000).ChannelName)
	assert.Equal(t, "glm-mid", matchLengthRoute(setting, "glm-5", 1999).ChannelName)
	// 上界开区间，2000 不再命中 mid
	assert.Equal(t, "glm-short", matchLengthRoute(setting, "glm-5", 2000).ChannelName)
	// 长上下文优先走专用渠道
	assert.Equal(t, "glm-long", matchLengthRoute(setting, "glm-5", 10000).ChannelName)
	assert.Equal(t, "glm-long", matchLengthRoute(setting, "glm-5", 500000).ChannelName)

	// 模型名精确匹配（大小写不敏感），其他模型不分流
	assert.Equal(t, "glm-long", matchLengthRoute(setting, "GLM-5", 20000).ChannelName)
	assert.Nil(t, matchLengthRoute(setting, "qwen3", 100))
}

func TestMatchLengthRouteEmptyModelNameMatchesAll(t *testing.T) {
	setting := &operation_setting.ChannelRuntimeSetting{
		LengthRoutingRules: []operation_setting.ChannelLengthRoute{
			{ModelName: "", ChannelName: "shared", MinLength: 0, MaxLength: 0, Enabled: true},
			{ModelName: "glm-5", ChannelName: "", MinLength: 0, MaxLength: 0, Enabled: true},
		},
	}
	operation_setting.NormalizeChannelRuntimeSetting(setting)

	// 模型名留空表示对所有模型生效；渠道名为空的行直接忽略
	assert.Equal(t, "shared", matchLengthRoute(setting, "anything", 10).ChannelName)
}

func TestNormalizeChannelRuntimeSettingDefaults(t *testing.T) {
	setting := &operation_setting.ChannelRuntimeSetting{}
	operation_setting.NormalizeChannelRuntimeSetting(setting)

	assert.Equal(t, 10, setting.ScrapeIntervalSeconds)
	// 采集超时按秒，默认 3 秒
	assert.Equal(t, operation_setting.DefaultScrapeTimeoutSeconds, setting.ScrapeTimeoutSeconds)
	assert.Equal(t, 3, setting.ScrapeTimeoutSeconds)
	// 样本默认保留 12 个月（一年）
	assert.Equal(t, operation_setting.DefaultSampleRetentionMonths, setting.SampleRetentionMonths)
	assert.Equal(t, 12, setting.SampleRetentionMonths)
	// 最大 batch 默认 4
	assert.Equal(t, 4, setting.DefaultMaxBatch)
	assert.Equal(t, 3, setting.SaturationWaitingSamples)
	assert.Equal(t, 1, setting.SaturationReleaseSamples)
}

func TestIsChannelCollectedFollowsMaxBatchOverrides(t *testing.T) {
	setting := &operation_setting.ChannelRuntimeSetting{
		MaxBatchOverrides: []operation_setting.ChannelMaxBatchOverride{
			{ChannelName: "glm-h20-01", MaxBatch: 16},
			{ChannelName: "broken", MaxBatch: 0},
		},
	}
	// 配了最大 batch 的渠道才采集，未配置（或值非法）的一律跳过
	assert.True(t, setting.IsChannelCollected("glm-h20-01"))
	assert.True(t, setting.IsChannelCollected("GLM-H20-01"))
	assert.False(t, setting.IsChannelCollected("glm-h20-02"))
	assert.False(t, setting.IsChannelCollected("broken"))
}
