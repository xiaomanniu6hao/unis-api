package service

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/tokenkit"

	"github.com/stretchr/testify/assert"
)

// 中文请求按 o200k 计再乘校正系数：Estimate 的字符权重会把两千多 token
// 算到 4096 以上，让 [0,4096) 的短请求误命中长上下文规则。
func TestCountInputTokensUsesBPEProxyForUnknownFamily(t *testing.T) {
	unit := "今天天气很好，我们一起去公园散步吧。你昨天说的那本书我已经看完了。"
	cjk := strings.Repeat(unit, 120) // vLLM /tokenize 实测 2400 prompt tokens
	body := []byte(fmt.Sprintf(
		`{"model":"qwen3_8b","messages":[{"role":"user","content":"%s"}],"max_tokens":1}`,
		cjk,
	))

	got := countInputTokens("qwen3_8b", body)
	assert.InDelta(t, 2400, got, 300, "应与 vLLM 实测的 2400 token 相差不超过 12.5%%")
	assert.Less(t, got, 4096, "2400 token 的请求不能跨过 4096 边界")

	// 旧实现（Estimate）正是这个 bug 的来源，留在断言里防止回退：
	// 同样长度的中文，Estimate 会算到 4096 以上
	longer := []byte(strings.Repeat(unit, 175))
	assert.GreaterOrEqual(t, tokenkit.Estimate("qwen3_8b", string(longer)), 4096)
	assert.Less(t, countTextTokens("qwen3_8b", longer), 4096)
}

// 英文请求按 cl100k 计：o200k 会把 4507 token 的英文算成 4000，跌破 4096 边界。
func TestCountInputTokensUsesCL100kForLatinText(t *testing.T) {
	// 与联调脚本一致：22500 字符英文，vLLM 实测 4507 prompt tokens
	en := strings.Repeat("hello world testing the quick brown fox jumps", 3000)[:22500]
	assert.InDelta(t, 4507, countInputTokens("qwen3_8b", []byte(en)), 120)

	content := strings.Repeat("hello world testing the quick brown fox jumps ", 100)
	body := []byte(fmt.Sprintf(
		`{"model":"qwen3_8b","messages":[{"role":"user","content":"%s"}],"max_tokens":1}`,
		content,
	))
	// 请求体小于精确计数上限，走不采样、不外推的路径
	assert.Less(t, len(body), lengthRouteExactCountBytes)
	// 只算 content，外加每条消息约 4 个模板 token（max_tokens 等字段不计入）
	assert.Equal(t, tokenkit.Count("gpt-4", content)+4, countInputTokens("qwen3_8b", body))
}

// OpenAI 家族仍按自己的 tiktoken 编码计数，不走代理编码。
func TestCountInputTokensKeepsOpenAIEncoding(t *testing.T) {
	content := strings.Repeat("hello world testing ", 400)
	body := []byte(fmt.Sprintf(
		`{"model":"gpt-4o","messages":[{"role":"user","content":"%s"}],"max_tokens":1}`,
		content,
	))
	assert.Equal(t, tokenkit.Count("gpt-4o", content)+4, countInputTokens("gpt-4o", body))
}

// prompt 直接给分词后的 token id 数组时，数组长度就是 prompt token 数。
// 把 id 当数字文本 BPE 会把 2048 个 id 算成 6000 上下，短请求全跑进长上下文渠道。
func TestCountInputTokensCountsTokenIDPromptByLength(t *testing.T) {
	ids := make([]string, 2048)
	for i := range ids {
		ids[i] = fmt.Sprintf("%d", 1000+i*37)
	}
	body := []byte(fmt.Sprintf(
		`{"model":"qwen3_8b","prompt":[%s],"max_tokens":512}`,
		strings.Join(ids, ","),
	))

	assert.Equal(t, 2048, countInputTokens("qwen3_8b", body))
	// 旧实现（整包 BPE）正是这个 bug 的来源，留在断言里防止回退
	assert.Greater(t, tokenkit.Count(lengthRouteLatinModel, string(body)), 4096)
}

// 请求体里的 \uXXXX 转义必须先反转义再计数：
// 客户端把中文写成 测 时，整包 BPE 会把 1 个汉字算成 3~4 个 token。
func TestCountInputTokensCountsUnescapedJSONText(t *testing.T) {
	unit := "今天天气很好，我们一起去公园散步吧。你昨天说的那本书我已经看完了。"
	content := strings.Repeat(unit, 120) // 同上，vLLM 实测 2400
	body := []byte(fmt.Sprintf(
		`{"model":"qwen3_8b","prompt":"%s","max_tokens":512}`,
		jsonEscapeNonASCII(content),
	))

	got := countInputTokens("qwen3_8b", body)
	assert.InDelta(t, 2400, got, 300, "应按反转义后的真实字符计数")
	assert.Less(t, got, 4096)

	// 旧实现把转义序列当原文计数，同一个请求会跨过 4096 边界
	assert.Greater(t, tokenkit.Count(lengthRouteLatinModel, string(body)), 4096)
}

// jsonEscapeNonASCII 把非 ASCII 字符写成 \uXXXX，模拟部分客户端的 JSON 编码方式。
func jsonEscapeNonASCII(text string) string {
	var b strings.Builder
	for _, r := range text {
		if r < utf8.RuneSelf {
			b.WriteRune(r)
			continue
		}
		fmt.Fprintf(&b, `\u%04x`, r)
	}
	return b.String()
}

// 超过精确计数上限的请求体按前 8KB 采样后外推，误差要足够小。
func TestCountInputTokensExtrapolatesLargeBody(t *testing.T) {
	unit := `测试文本内容，hello world. `
	big := []byte(strings.Repeat(unit, 3000))  // ≈ 100KB
	huge := []byte(strings.Repeat(unit, 6000)) // ≈ 200KB
	assert.Greater(t, len(big), lengthRouteScriptSampleBytes)

	// 采样外推必须线性：两倍文本就是两倍 token
	gotBig := countInputTokens("qwen3_8b", big)
	gotHuge := countInputTokens("qwen3_8b", huge)
	assert.InDelta(t, 2*gotBig, gotHuge, float64(gotHuge)*0.02)

	// 采样窗口之外的部分按比例计入，而不是只算前 8KB
	assert.Greater(t, gotBig, 10*countInputTokens("qwen3_8b", []byte(strings.Repeat(unit, 8))))
}

// 采样截断点落在多字节字符中间时不能把半个字符算成一个 token。
func TestCountInputTokensHandlesSplitRuneAtSampleBoundary(t *testing.T) {
	// 8KB 不是 3 的整数倍，这里必然切在某个汉字的中间
	body := []byte(strings.Repeat("测", 20000))
	assert.NotEqual(t, 0, lengthRouteScriptSampleBytes%3)

	got := countInputTokens("qwen3_8b", body)
	// 20000 个汉字（o200k 基本一字一 token）乘校正系数，误差只来自截断
	assert.InDelta(t, lengthRouteCJKScale*20000, got, 400)
	assert.Greater(t, got, 0)
}

// 生僻字 / 随机码位文本：o200k 会把一个字拆成近 2 个 token，
// 而 qwen 这类大词表分词器基本一字一 token。统一乘校正系数后，
// vllm bench 的随机文本（真实 799~4881）不会再被算到高出一倍。
func TestCountTextTokensScalesCJKByProxy(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 2048; i++ {
		b.WriteRune(rune(0x4E00 + (i*97)%20000))
	}
	text := []byte(b.String())

	raw := tokenkit.Count(lengthRouteCJKModel, string(text))
	assert.Greater(t, raw, 2048, "o200k 对随机汉字明显高估，正是需要校正的场景")

	got := countTextTokens("qwen3_8b", text)
	assert.Less(t, got, raw)
	assert.Greater(t, got, int(float64(raw)*0.75))
}

// GLM、DeepSeek 等厂商 tokenkit 也认不出词表，走同一条代理编码路径；
// 分支依据是 tokenkit.ModelFamily，不是硬编码的模型名。
func TestCountTextTokensTreatsOtherVendorsAsUnknownFamily(t *testing.T) {
	text := []byte(strings.Repeat("渠道长度分流根据输入长度选择渠道。", 40))
	want := countTextTokens("qwen3_8b", text)
	for _, model := range []string{"glm-5.3", "deepseek-v4.1-flash"} {
		assert.Equal(t, want, countTextTokens(model, text), model)
	}
	// OpenAI 家族仍按自己的 tiktoken 编码
	assert.Equal(t, tokenkit.Count("gpt-4o", string(text)), countTextTokens("gpt-4o", text))
}

// 混合文本按文字拆开计数：中文部分用 o200k（乘校正系数）、英文部分用 cl100k，
// 不能整篇用一种编码（会把其中一半高估或低估）。
func TestCountTextTokensSplitsMixedScripts(t *testing.T) {
	cjk := strings.Repeat("测试文本内容", 20)                     // 120 汉字
	en := strings.Repeat("hello world testing the fox ", 5) // 150 字符
	mixed := cjk + en + cjk

	// 中文、英文各算各的，合起来就是总数
	assert.Equal(t,
		countTextTokens("qwen3_8b", []byte(cjk))+countTextTokens("qwen3_8b", []byte(en))+countTextTokens("qwen3_8b", []byte(cjk)),
		countTextTokens("qwen3_8b", []byte(mixed)))
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
