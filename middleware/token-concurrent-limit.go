package middleware

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
)

// tokenInFlight 统计每个API密钥（令牌）正在处理中的请求数。
// 仅存内存：defer 释放保证不泄漏（panic 时 defer 随中间件栈展开执行，进程重启自动清零）。
// 多节点部署时该上限按节点各自计算。
var tokenInFlight = struct {
	sync.Mutex
	counts map[int]int
}{counts: make(map[int]int)}

// TokenConcurrentLimit 限制每个API密钥同时进行中的请求数，超出返回 429。
// 必须挂在 TokenAuth 之后（依赖 token_id 上下文）、Distribute 之前。
func TokenConcurrentLimit() func(c *gin.Context) {
	return func(c *gin.Context) {
		maxCount := setting.TokenConcurrentLimitMaxCount
		if !setting.TokenConcurrentLimitEnabled || maxCount <= 0 {
			c.Next()
			return
		}
		tokenId := c.GetInt("token_id")
		if tokenId <= 0 || setting.IsTokenConcurrentLimitExcluded(tokenId) {
			c.Next()
			return
		}

		tokenInFlight.Lock()
		if tokenInFlight.counts[tokenId] >= maxCount {
			tokenInFlight.Unlock()
			abortWithOpenAiMessage(c, http.StatusTooManyRequests, fmt.Sprintf("当前API密钥同时进行的请求数已达上限（%d），请稍后重试", maxCount))
			return
		}
		tokenInFlight.counts[tokenId]++
		tokenInFlight.Unlock()

		defer func() {
			tokenInFlight.Lock()
			tokenInFlight.counts[tokenId]--
			if tokenInFlight.counts[tokenId] <= 0 {
				delete(tokenInFlight.counts, tokenId)
			}
			tokenInFlight.Unlock()
		}()

		c.Next()
	}
}
