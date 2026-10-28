package setting

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

var ModelRequestRateLimitEnabled = false
var ModelRequestRateLimitDurationMinutes = 1
var ModelRequestRateLimitCount = 0
var ModelRequestRateLimitSuccessCount = 1000
var ModelRequestRateLimitGroup = map[string][2]int{}
var ModelRequestRateLimitMutex sync.RWMutex

// TokenConcurrentLimit 限制每个API密钥（令牌）同时进行中的请求数，
// 高峰期可通过后台开关临时启用，超出上限的请求直接返回 429。
var TokenConcurrentLimitEnabled = false
var TokenConcurrentLimitMaxCount = 2

// TokenConcurrentLimitRetryAfterSeconds 并发超限 429 响应里 Retry-After 头的秒数，
// 即建议客户端等待多久再重试；小于 1 的配置在中间件里按 1 秒兜底。
var TokenConcurrentLimitRetryAfterSeconds = 5

// TokenConcurrentLimitModelMaxCountOverrides 按模型覆盖的并发上限（模型名 → 最大并发数，精确匹配），
// 未覆盖的模型使用 TokenConcurrentLimitMaxCount 默认值。
var TokenConcurrentLimitModelMaxCountOverrides = map[string]int{}
var TokenConcurrentLimitModelMaxCountOverridesMutex sync.RWMutex

// TokenConcurrentLimitExcludedTokenIds 并发限制排除的令牌 ID 列表（逗号分隔），
// 公共令牌（如 rerank、embed、lobehub 共享 key）填入后不受并发限制。
var TokenConcurrentLimitExcludedTokenIds = ""

// IsTokenConcurrentLimitExcluded 判断令牌是否在并发限制排除列表中。
func IsTokenConcurrentLimitExcluded(tokenId int) bool {
	if tokenId <= 0 || TokenConcurrentLimitExcludedTokenIds == "" {
		return false
	}
	for _, part := range strings.Split(TokenConcurrentLimitExcludedTokenIds, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.Atoi(part)
		if err == nil && id == tokenId {
			return true
		}
	}
	return false
}

// TokenConcurrentLimitDefaultExcludedPatterns 排除列表首次初始化时按名称子串匹配
// （不区分大小写）的公共共享令牌关键词，与数据看板筛选的默认排除关键词一致。
var TokenConcurrentLimitDefaultExcludedPatterns = []string{"rerank", "embed", "lobehub"}

// IsTokenConcurrentLimitDefaultExcludedName 判断令牌名称是否命中默认排除关键词，
// 用于并发限制排除列表的首次初始化。
func IsTokenConcurrentLimitDefaultExcludedName(name string) bool {
	name = strings.ToLower(name)
	for _, pattern := range TokenConcurrentLimitDefaultExcludedPatterns {
		if strings.Contains(name, pattern) {
			return true
		}
	}
	return false
}

// TokenConcurrentLimitModelMaxCountOverrides2JSONString 导出按模型覆盖的并发上限配置。
func TokenConcurrentLimitModelMaxCountOverrides2JSONString() string {
	TokenConcurrentLimitModelMaxCountOverridesMutex.RLock()
	defer TokenConcurrentLimitModelMaxCountOverridesMutex.RUnlock()

	jsonBytes, err := common.Marshal(TokenConcurrentLimitModelMaxCountOverrides)
	if err != nil {
		common.SysLog("error marshalling token concurrent model overrides: " + err.Error())
	}
	return string(jsonBytes)
}

// UpdateTokenConcurrentLimitModelMaxCountOverridesByJSONString 解析并替换按模型覆盖的并发上限配置。
func UpdateTokenConcurrentLimitModelMaxCountOverridesByJSONString(jsonStr string) error {
	overrides := make(map[string]int)
	if err := common.Unmarshal([]byte(jsonStr), &overrides); err != nil {
		return err
	}
	for model, count := range overrides {
		if count < 1 || count > 1000 {
			return fmt.Errorf("model %s has invalid concurrency limit: %d", model, count)
		}
	}
	TokenConcurrentLimitModelMaxCountOverridesMutex.Lock()
	defer TokenConcurrentLimitModelMaxCountOverridesMutex.Unlock()
	TokenConcurrentLimitModelMaxCountOverrides = overrides
	return nil
}

// GetTokenConcurrentLimitMaxCount 返回模型对应的最大并发数：覆盖值优先，未覆盖用默认值。
func GetTokenConcurrentLimitMaxCount(modelName string) int {
	TokenConcurrentLimitModelMaxCountOverridesMutex.RLock()
	count, found := TokenConcurrentLimitModelMaxCountOverrides[modelName]
	TokenConcurrentLimitModelMaxCountOverridesMutex.RUnlock()
	if found && count >= 1 {
		return count
	}
	return TokenConcurrentLimitMaxCount
}

func ModelRequestRateLimitGroup2JSONString() string {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	jsonBytes, err := json.Marshal(ModelRequestRateLimitGroup)
	if err != nil {
		common.SysLog("error marshalling model ratio: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateModelRequestRateLimitGroupByJSONString(jsonStr string) error {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	ModelRequestRateLimitGroup = make(map[string][2]int)
	return json.Unmarshal([]byte(jsonStr), &ModelRequestRateLimitGroup)
}

func GetGroupRateLimit(group string) (totalCount, successCount int, found bool) {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	if ModelRequestRateLimitGroup == nil {
		return 0, 0, false
	}

	limits, found := ModelRequestRateLimitGroup[group]
	if !found {
		return 0, 0, false
	}
	return limits[0], limits[1], true
}

func CheckModelRequestRateLimitGroup(jsonStr string) error {
	checkModelRequestRateLimitGroup := make(map[string][2]int)
	err := json.Unmarshal([]byte(jsonStr), &checkModelRequestRateLimitGroup)
	if err != nil {
		return err
	}
	for group, limits := range checkModelRequestRateLimitGroup {
		if limits[0] < 0 || limits[1] < 1 {
			return fmt.Errorf("group %s has negative rate limit values: [%d, %d]", group, limits[0], limits[1])
		}
		if limits[0] > math.MaxInt32 || limits[1] > math.MaxInt32 {
			return fmt.Errorf("group %s [%d, %d] has max rate limits value 2147483647", group, limits[0], limits[1])
		}
	}

	return nil
}
