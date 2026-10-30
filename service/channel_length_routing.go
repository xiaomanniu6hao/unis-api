package service

import (
	"fmt"
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
	"github.com/tidwall/gjson"
)

// lengthRouteMaxBodyBytes 是估算输入长度时读取请求体的上限。
// 超过上限的请求体一定属于超长上下文，直接按「无上界」档处理，避免为超大体做无谓遍历。
const lengthRouteMaxBodyBytes = 8 << 20

// 长度估算用的 BPE 代理编码。qwen 等模型的 tokenizer 家族 tokenkit 认不出来，
// 原本走的是按字符权重累加的 Estimate：中文每个汉字按 0.85 token 计，
// 实测比 Qwen 分词高估约 1.7 倍（4800 汉字：Estimate 4080，真实 2408），
// 于是 2400 token 的请求被算成 4100，跨过 4096 的边界跑到了长上下文渠道。
//
// 两个真实编码各擅长一半，按文字拆开各用各的：
//   - 中文：o200k 最接近，但仍高估，再乘 lengthRouteCJKScale 校正
//   - 英文/代码：cl100k 与 qwen 实测一致（误差 0.0%~0.1%）
const (
	lengthRouteCJKModel   = "gpt-4o" // o200k_base
	lengthRouteLatinModel = "gpt-4"  // cl100k_base
	// lengthRouteCJKScale 把 o200k 的中文计数校正到 qwen 的量级。
	// 用 vLLM /tokenize 对照 19 条语料实测：真实值与 o200k 的比值稳定在
	// 0.77（日常对话）~0.88（说明文）、随机码位 0.82 上下；统一乘 0.82 后
	// 中文误差收敛到 -4%~+9%，英文与代码本来就在 ±0.1%。
	// GLM、DeepSeek 等厂商没有本地可用的词表，同样走这条代理路径，
	// 拿到它们的 /tokenize 后可以用同一套语料重新校准这个系数。
	lengthRouteCJKScale = 0.82
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
// 这些字符在两个候选编码下的计数量级差异最大，用来决定两种编码的权重。
func isCJKEncodingRune(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r)
}

// lengthRouteScriptSampleBytes 是每类文字单独抽样计数的上限。
// 两类文字各取这么多字节，既保证总量可控（最多 16KB），又够贴近真实分词比例。
const lengthRouteScriptSampleBytes = 8 << 10

// scriptSample 是一类文字（CJK 或非 CJK）的抽样：text 是抽到的字节，
// size 是这类文字在全文中占的字节数（用来按比例外推）。
type scriptSample struct {
	text []byte
	size int
}

// sampleByScript 把文本按文字分成 CJK 与非 CJK 两份，各自只保留同类字节，
// 两类文字再各用最贴近的编码计数：
//   - 中文/日文/韩文 → o200k（与 Qwen 同一量级，再乘 lengthRouteCJKScale）
//   - 其余（英文、代码、标点）→ cl100k（英文实测一致）
//
// 不能整篇只挑一种编码：cl100k 数中文会高估 50%，o200k 数英文会略低；
// 也不能按连续段落切分再各算各的：一个标点就会把后面整段中文归到英文编码下。
func sampleByScript(text []byte) (cjk, latin scriptSample) {
	cjk.text = make([]byte, 0, min(len(text), lengthRouteScriptSampleBytes))
	latin.text = make([]byte, 0, min(len(text), lengthRouteScriptSampleBytes))

	flush := func(start, end int, runIsCJK bool) {
		n := end - start
		sample := &latin
		if runIsCJK {
			sample = &cjk
		}
		sample.size += n
		// 只取前 8KB 参与分词，剩下的按字节比例外推。
		if room := lengthRouteScriptSampleBytes - len(sample.text); room > 0 {
			if n > room {
				n = room
			}
			sample.text = append(sample.text, text[start:start+n]...)
		}
	}

	start := 0
	isCJK := false
	first := true
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRune(text[i:])
		runeIsCJK := isCJKEncodingRune(r)
		if !runeIsCJK && unicode.IsSpace(r) {
			// 空白跟着前一段走：分词器是按「空格 + 词」编码的，
			// 把「模型推理 」的空格留在这里，中文段才能按真实方式切分。
			runeIsCJK = isCJK
		}
		if first {
			isCJK, first = runeIsCJK, false
		} else if runeIsCJK != isCJK {
			flush(start, i, isCJK)
			start, isCJK = i, runeIsCJK
		}
		i += size
	}
	flush(start, len(text), isCJK)
	// 截断点可能落在多字节字符中间，退回完整字符再分词。
	cjk.text = trimRuneBoundary(cjk.text)
	latin.text = trimRuneBoundary(latin.text)
	return cjk, latin
}

// trimRuneBoundary 去掉末尾被截断的半个字符。
func trimRuneBoundary(b []byte) []byte {
	for len(b) > 0 {
		if r, size := utf8.DecodeLastRune(b); r != utf8.RuneError || size > 1 {
			break
		}
		b = b[:len(b)-1]
	}
	return b
}

// countInputTokens 估算 prompt token 数。
// 先按请求体的内容字段（prompt / messages / input）计数，识别不出来才退回整包估算。
// 内容字段能拿到的两个收益：
//   - prompt 直接给分词后的 token id 数组时，数组长度就是 prompt token 数，
//     不用再把 2048 个 id 当数字文本 BPE 一遍（会被算成 4000~8000，短请求跑进长渠道）；
//   - max_tokens、temperature、response_format 等非 prompt 字段不再计入。
func countInputTokens(modelName string, body []byte) int {
	if tokens, ok := countContentTokens(modelName, body); ok {
		return tokens
	}
	return countTextTokens(modelName, body)
}

// countContentTokens 按 OpenAI 兼容协议的内容字段计数。
// ok=false 表示请求体里没有可识别的 prompt/messages/input（例如透传的私有协议）。
func countContentTokens(modelName string, body []byte) (int, bool) {
	if prompt := gjson.GetBytes(body, "prompt"); prompt.Exists() {
		if tokens, ok := countPromptValue(modelName, prompt); ok {
			return tokens, true
		}
	}
	if messages := gjson.GetBytes(body, "messages"); messages.IsArray() {
		total := countMessageList(modelName, messages)
		if tools := gjson.GetBytes(body, "tools"); tools.IsArray() {
			// 工具定义的 JSON 也是模型实际看到的 prompt，按文本计一遍。
			total += countTextTokens(modelName, []byte(tools.Raw))
		}
		return total, true
	}
	if input := gjson.GetBytes(body, "input"); input.Exists() {
		if tokens, ok := countPromptValue(modelName, input); ok {
			return tokens, true
		}
	}
	return 0, false
}

// countPromptValue 处理 prompt / input 字段的三种形态：字符串、字符串数组、token id 数组。
func countPromptValue(modelName string, value gjson.Result) (int, bool) {
	if value.Type == gjson.String {
		return countTextTokens(modelName, []byte(value.String())), true
	}
	if value.IsArray() {
		return countPromptArray(modelName, value), true
	}
	return 0, false
}

func countPromptArray(modelName string, value gjson.Result) int {
	total := 0
	value.ForEach(func(_, item gjson.Result) bool {
		switch {
		case item.Type == gjson.Number:
			// token id 数组：每个 id 就是一个 prompt token。
			total++
		case item.IsArray():
			total += countPromptArray(modelName, item)
		default:
			total += countTextTokens(modelName, []byte(item.String()))
		}
		return true
	})
	return total
}

func countMessageList(modelName string, messages gjson.Result) int {
	total := 0
	messages.ForEach(func(_, message gjson.Result) bool {
		// 角色标记与消息分隔符在 chat template 里约占 4 个 token。
		total += 4
		content := message.Get("content")
		switch {
		case content.Type == gjson.String:
			total += countTextTokens(modelName, []byte(content.String()))
		case content.IsArray():
			content.ForEach(func(_, part gjson.Result) bool {
				// 图片/音频等非文本部分不计入长度分流。
				if text := part.Get("text"); text.Exists() {
					total += countTextTokens(modelName, []byte(text.String()))
				}
				return true
			})
		}
		if toolCalls := message.Get("tool_calls"); toolCalls.Exists() {
			total += countTextTokens(modelName, []byte(toolCalls.Raw))
		}
		return true
	})
	return total
}

// countTextTokens 统计一段纯文本的 token 数。
// OpenAI 家族按自己的 tiktoken 编码精确计数；其他模型（qwen、glm 等）
// tokenkit 认不出它们的词表，按文字拆开各用最贴近的代理编码，比 Estimate
// 的字符权重启发式贴近真实分词（实测中文 ±8%、英文与代码 ±0.1%）。
func countTextTokens(modelName string, text []byte) int {
	if len(text) == 0 {
		return 0
	}
	if tokenkit.ModelFamily(modelName) == tokenkit.FamilyOpenAI {
		return countSegmentTokens(modelName, text, 1)
	}
	cjk, latin := sampleByScript(text)
	return countSampleTokens(lengthRouteCJKModel, cjk.text, cjk.size, lengthRouteCJKScale) +
		countSampleTokens(lengthRouteLatinModel, latin.text, latin.size, 1)
}

// countSegmentTokens 用指定编码统计一段文本，再按 scale 校正到目标模型的量级。
// 超过精确计数上限的文本取前 16KB 计数后按字节比例外推。
func countSegmentTokens(encoding string, text []byte, scale float64) int {
	if len(text) == 0 {
		return 0
	}
	whole := len(text)
	if whole > lengthRouteExactCountBytes {
		whole = lengthRouteExactCountBytes
	}
	sample := trimRuneBoundary(text[:whole])
	if len(sample) == 0 {
		return lengthRouteExactCountBytes
	}
	return countSampleTokens(encoding, sample, len(text), scale)
}

// countSampleTokens 用 sample 的 BPE 计数按字节比例外推到 totalBytes。
func countSampleTokens(encoding string, sample []byte, totalBytes int, scale float64) int {
	if len(sample) == 0 || totalBytes == 0 {
		return 0
	}
	tokens := float64(tokenkit.Count(encoding, string(sample))) * scale
	if len(sample) == totalBytes {
		return int(math.Round(tokens))
	}
	return int(math.Round(tokens * float64(totalBytes) / float64(len(sample))))
}

// estimateRequestInputLength 估算请求输入的 prompt token 数（长度单位固定为 token）。
// 读不到请求体时返回 false，由调用方跳过分流。
func estimateRequestInputLength(c *gin.Context, modelName string) (int, bool) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		common.SysLog(fmt.Sprintf("长度分流：读取请求体失败，跳过分流 (model=%s): %v", modelName, err))
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
