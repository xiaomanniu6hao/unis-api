package middleware

import (
	"fmt"
	"sync"

	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

// tokenInFlight 统计每个API密钥（令牌）按模型维度的在途请求数：
// counts[tokenId][modelName] = 该令牌该模型正在处理中的请求数。
// 仅存内存：defer 释放保证不泄漏（panic 时 defer 随中间件栈展开执行，进程重启自动清零）。
// 多节点部署时该上限按节点各自计算。
var tokenInFlight = struct {
	sync.Mutex
	counts map[int]map[string]int
}{counts: make(map[int]map[string]int)}

// TokenConcurrentLimit 限制每个API密钥同时进行中的请求数（按模型分别计数），超出返回 429。
// 必须挂在 TokenAuth 之后（依赖 token_id 上下文）、Distribute 之前。
func TokenConcurrentLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		if !setting.TokenConcurrentLimitEnabled {
			c.Next()
			return
		}
		tokenId := c.GetInt("token_id")
		if tokenId <= 0 || setting.IsTokenConcurrentLimitExcluded(tokenId) {
			c.Next()
			return
		}

		// 模型名解析失败时按空模型名计数（退化为令牌级默认限制），解析错误交给后续 Distribute 处理。
		// getModelRequest 可安全重复调用（请求体可重放，Distribute 内会再次调用）。
		modelName := ""
		if modelRequest, _, err := getModelRequest(c); err == nil && modelRequest != nil {
			modelName = modelRequest.Model
		}
		maxCount := setting.GetTokenConcurrentLimitMaxCount(modelName)
		if maxCount <= 0 {
			c.Next()
			return
		}

		tokenInFlight.Lock()
		modelCounts := tokenInFlight.counts[tokenId]
		if modelCounts == nil {
			modelCounts = make(map[string]int)
			tokenInFlight.counts[tokenId] = modelCounts
		}
		if modelCounts[modelName] >= maxCount {
			tokenInFlight.Unlock()
			retryAfter := setting.TokenConcurrentLimitRetryAfterSeconds
			if retryAfter < 1 {
				retryAfter = 1
			}
			message := fmt.Sprintf("当前API密钥同时进行的请求数已达上限（%d），请降低并发或稍后重试", maxCount)
			if modelName != "" {
				message = fmt.Sprintf("当前API密钥对模型 %s 的并发请求数已达上限（%d），请降低并发或稍后重试", modelName, maxCount)
			}
			abortWithRateLimitMessage(c, message, retryAfter)
			return
		}
		modelCounts[modelName]++
		tokenInFlight.Unlock()

		defer func() {
			tokenInFlight.Lock()
			modelCounts := tokenInFlight.counts[tokenId]
			if modelCounts != nil {
				modelCounts[modelName]--
				if modelCounts[modelName] <= 0 {
					delete(modelCounts, modelName)
				}
				if len(modelCounts) == 0 {
					delete(tokenInFlight.counts, tokenId)
				}
			}
			tokenInFlight.Unlock()
		}()

		c.Next()
	}
}
