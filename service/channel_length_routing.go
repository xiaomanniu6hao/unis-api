package service

import (
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/tokenkit"

	"github.com/gin-gonic/gin"
)

// lengthRouteMaxBodyBytes 是估算输入长度时读取请求体的上限。
// 超过上限的请求体一定属于超长上下文，直接按「无上界」档处理，避免为超大体做无谓遍历。
const lengthRouteMaxBodyBytes = 8 << 20

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
	return tokenkit.Estimate(modelName, string(body)), true
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
