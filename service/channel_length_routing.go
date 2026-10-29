package service

import (
	"math"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/tokenkit"

	"github.com/gin-gonic/gin"
)

// lengthRouteMaxBodyBytes 是估算输入长度时读取请求体的上限。
// 超过上限的请求体一定属于超长上下文，直接按「无上界」档处理，避免为超大体做无谓遍历。
const lengthRouteMaxBodyBytes = 8 << 20

// 长度估算用的 BPE 代理编码。qwen 等模型的 tokenizer 家族 tokenkit 认不出来，
// 原本走的是按字符权重累加的 Estimate：中文每个汉字按 0.85 token 计，
// 实测比 Qwen 分词高估约 1.7 倍（4800 汉字：Estimate 4080，真实 2408），
// 于是 2400 token 的请求被算成 4100，跨过 4096 的边界跑到了长上下文渠道。
//
// 两个真实编码各擅长一半，按 CJK 字符占比二选一：
//   - 中文：o200k 2400 vs cl100k 3200，真实 2408  → o200k 几乎精确
//   - 英文：o200k 4000 vs cl100k 4499，真实 4507  → cl100k 几乎精确
const (
	lengthRouteCJKModel   = "gpt-4o" // o200k_base
	lengthRouteLatinModel = "gpt-4"  // cl100k_base
	// CJK 字符占比达到这个比例就按中文编码计数。纯中文/纯英文都能精确命中，
	// 混合文本最坏偏差约 15%（选了反而更不贴的那个编码）。
	lengthRouteCJKShareThreshold = 0.2
)

// lengthRouteExactCountBytes 是全文精确计数的上限。
// o200k 纯 Go 实现约 1MB/s，8MB 请求体要 11 秒，不能全文扫。
// 分流边界附近的请求体通常只有几 KB，能拿到精确值；
// 远超上限的请求体只取前 16KB 计数后按字节比例外推，量级足够区分长短。
const lengthRouteExactCountBytes = 16 << 10

// lengthRouteChannelCacheTTL 控制「渠道名 → id」解析结果的缓存时长：
// 避免每个请求都查库，同时让渠道重命名/删除在一分钟内生效。
const lengthRouteChannelCacheTTL = 60 * time.Second

type lengthRouteChannelRef struct {
	id      int
	expires time.Time
}

var lengthRouteChannelCache sync.Map // lower(channel_name) -> lengthRouteChannelRef

// resolveLengthRouteChannelId 把规则里的渠道名解析成渠道 id（带 TTL 缓存，未命中不缓存）。
func resolveLengthRouteChannelId(channelName string) (int, bool) {
	name := strings.TrimSpace(channelName)
	if name == "" {
		return 0, false
	}
	key := strings.ToLower(name)
	if cached, ok := lengthRouteChannelCache.Load(key); ok {
		ref, _ := cached.(lengthRouteChannelRef)
		if time.Now().Before(ref.expires) {
			return ref.id, ref.id > 0
		}
	}
	id, err := model.GetChannelIdByName(name)
	if err != nil {
		common.SysError("length routing: resolve channel " + name + " failed: " + err.Error())
		return 0, false
	}
	if id <= 0 {
		return 0, false
	}
	lengthRouteChannelCache.Store(key, lengthRouteChannelRef{
		id:      id,
		expires: time.Now().Add(lengthRouteChannelCacheTTL),
	})
	return id, true
}

// matchLengthRoute 找出该模型在该输入长度下最具体的规则：区间下界最大的那条。
// 这样 [0,0]（全量兜底）和 [10000,0]（长上下文专用）同时存在时，长请求优先走专用渠道。
func matchLengthRoute(
	setting *operation_setting.ChannelRuntimeSetting,
	modelName string,
	length int,
) *operation_setting.ChannelLengthRoute {
	var matched *operation_setting.ChannelLengthRoute
	for i := range setting.LengthRoutingRules {
		route := &setting.LengthRoutingRules[i]
		if !route.Enabled || strings.TrimSpace(route.ChannelName) == "" {
			continue
		}
		routeModel := strings.TrimSpace(route.ModelName)
		if routeModel != "" && !strings.EqualFold(routeModel, modelName) {
			continue
		}
		if length < route.MinLength {
			continue
		}
		if route.MaxLength > 0 && length >= route.MaxLength {
			continue
		}
		if matched == nil || route.MinLength > matched.MinLength {
			matched = route
		}
	}
	return matched
}

// isCJKEncodingRune 判断字符是否属于 CJK 文字（汉字、假名、谚文）：
// 这些字符在两个候选编码下差异最大，用来决定选哪个编码计数。
func isCJKEncodingRune(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r)
}

// countCJKShare 统计 CJK 字符占全部字符的比例，用于选择代理编码。
func countCJKShare(text []byte) float64 {
	total, cjk := 0, 0
	for len(text) > 0 {
		r, size := utf8.DecodeRune(text)
		text = text[size:]
		total++
		if isCJKEncodingRune(r) {
			cjk++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(cjk) / float64(total)
}

// proxyEncodingModel 选择非 OpenAI 模型的计数编码：CJK 为主用 o200k，
// 否则用 cl100k（贴近 Qwen 对英文的分词效率）。
func proxyEncodingModel(modelName string, sample []byte) string {
	if tokenkit.ModelFamily(modelName) == tokenkit.FamilyOpenAI {
		return modelName
	}
	if countCJKShare(sample) >= lengthRouteCJKShareThreshold {
		return lengthRouteCJKModel
	}
	return lengthRouteLatinModel
}

// countInputTokens 估算 prompt token 数。
// OpenAI 家族按自己的 tiktoken 编码精确计数；其他模型（qwen、glm 等）
// 用与文本语种匹配的代理编码，比 Estimate 的字符权重启发式贴近真实分词。
func countInputTokens(modelName string, body []byte) int {
	if len(body) <= lengthRouteExactCountBytes {
		return tokenkit.Count(proxyEncodingModel(modelName, body), string(body))
	}
	sample := body[:lengthRouteExactCountBytes]
	// 截断点可能切在多字节字符中间，退到完整的 rune 边界再计数。
	for len(sample) > 0 {
		if r, size := utf8.DecodeLastRune(sample); r != utf8.RuneError || size > 1 {
			break
		}
		sample = sample[:len(sample)-1]
	}
	if len(sample) == 0 {
		return lengthRouteExactCountBytes
	}
	sampled := tokenkit.Count(proxyEncodingModel(modelName, sample), string(sample))
	return int(math.Round(float64(sampled) * float64(len(body)) / float64(len(sample))))
}

// estimateRequestInputLength 估算请求输入的 prompt token 数（长度单位固定为 token）。
// 读不到请求体时返回 false，由调用方跳过分流。
func estimateRequestInputLength(c *gin.Context, modelName string) (int, bool) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return 0, false
	}
	body, err := storage.Bytes()
	if err != nil || len(body) == 0 {
		return 0, false
	}
	if len(body) > lengthRouteMaxBodyBytes {
		return lengthRouteMaxBodyBytes, true
	}
	return countInputTokens(modelName, body), true
}

// ResolveLengthRouteChannelIds 按「模型 + 输入长度」解析出允许调度的渠道集合。
// 返回 ok=false 表示未命中分流（不做任何限制）。
func ResolveLengthRouteChannelIds(c *gin.Context, modelName string) ([]int, bool) {
	if c == nil {
		return nil, false
	}
	setting := operation_setting.GetChannelRuntimeSetting()
	if setting == nil || !setting.Enabled || !setting.LengthRoutingEnabled {
		return nil, false
	}
	operation_setting.NormalizeChannelRuntimeSetting(setting)

	length, ok := estimateRequestInputLength(c, modelName)
	if !ok {
		return nil, false
	}
	route := matchLengthRoute(setting, modelName, length)
	if route == nil {
		return nil, false
	}
	channelId, ok := resolveLengthRouteChannelId(route.ChannelName)
	if !ok {
		return nil, false
	}
	return []int{channelId}, true
}
