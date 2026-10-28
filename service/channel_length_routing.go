package service

import (
	"regexp"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// lengthRouteMaxBodyBytes 是估算输入长度时读取请求体的上限。
// 超过上限的请求体一定属于超长上下文，直接按「无上界」档处理，避免为超大体做无谓遍历。
const lengthRouteMaxBodyBytes = 8 << 20

var lengthRouteRegexCache sync.Map // pattern -> *regexp.Regexp

func compileLengthRouteRegex(pattern string) *regexp.Regexp {
	if compiled, ok := lengthRouteRegexCache.Load(pattern); ok {
		re, _ := compiled.(*regexp.Regexp)
		return re
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		common.SysError("invalid length routing model_regex: " + pattern)
		re = nil
	}
	lengthRouteRegexCache.Store(pattern, re)
	return re
}

// matchLengthRouteRule 返回第一个启用且模型正则命中的长度分流规则。
func matchLengthRouteRule(setting *operation_setting.ChannelRuntimeSetting, modelName string) *operation_setting.ChannelLengthRouteRule {
	for i := range setting.LengthRoutingRules {
		rule := &setting.LengthRoutingRules[i]
		if !rule.Enabled || len(rule.Segments) == 0 {
			continue
		}
		if len(rule.ModelRegex) == 0 {
			continue
		}
		for _, pattern := range rule.ModelRegex {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}
			if re := compileLengthRouteRegex(pattern); re != nil && re.MatchString(modelName) {
				return rule
			}
		}
	}
	return nil
}

// pickLengthSegment 找出输入长度落在哪个区间。区间为 [MinLength, MaxLength)，MaxLength <= 0 表示无上界。
func pickLengthSegment(rule *operation_setting.ChannelLengthRouteRule, length int) (*operation_setting.ChannelLengthSegment, bool) {
	for i := range rule.Segments {
		segment := &rule.Segments[i]
		if length < segment.MinLength {
			continue
		}
		if segment.MaxLength > 0 && length >= segment.MaxLength {
			continue
		}
		return segment, true
	}
	return nil, false
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
	return EstimateTokenByModel(modelName, string(body)), true
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

	rule := matchLengthRouteRule(setting, modelName)
	if rule == nil {
		return nil, false
	}
	length, ok := estimateRequestInputLength(c, modelName)
	if !ok {
		return nil, false
	}
	segment, ok := pickLengthSegment(rule, length)
	if !ok || len(segment.ChannelIds) == 0 {
		return nil, false
	}
	return segment.ChannelIds, true
}
