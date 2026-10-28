package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withTokenConcurrentLimitSettings(t *testing.T, enabled bool, maxCount, retryAfterSeconds int, modelOverrides map[string]int) {
	t.Helper()

	prevEnabled := setting.TokenConcurrentLimitEnabled
	prevMaxCount := setting.TokenConcurrentLimitMaxCount
	prevRetryAfter := setting.TokenConcurrentLimitRetryAfterSeconds
	prevOverrides := setting.TokenConcurrentLimitModelMaxCountOverrides
	setting.TokenConcurrentLimitEnabled = enabled
	setting.TokenConcurrentLimitMaxCount = maxCount
	setting.TokenConcurrentLimitRetryAfterSeconds = retryAfterSeconds
	setting.TokenConcurrentLimitModelMaxCountOverrides = modelOverrides
	t.Cleanup(func() {
		setting.TokenConcurrentLimitEnabled = prevEnabled
		setting.TokenConcurrentLimitMaxCount = prevMaxCount
		setting.TokenConcurrentLimitRetryAfterSeconds = prevRetryAfter
		setting.TokenConcurrentLimitModelMaxCountOverrides = prevOverrides
	})
}

type rateLimitErrorBody struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func decodeRateLimitBody(t *testing.T, recorder *httptest.ResponseRecorder) rateLimitErrorBody {
	t.Helper()

	var body rateLimitErrorBody
	require.NoError(t, common.DecodeJson(recorder.Body, &body))
	return body
}

func newChatCompletionRequest(modelName string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(fmt.Sprintf(`{"model":%q}`, modelName)))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestTokenConcurrentLimitRejectsSecondInFlightRequest(t *testing.T) {
	cases := []struct {
		name       string
		retryAfter int
		wantHeader string
	}{
		{name: "uses configured retry-after", retryAfter: 5, wantHeader: "5"},
		{name: "clamps invalid retry-after to 1", retryAfter: 0, wantHeader: "1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTokenConcurrentLimitSettings(t, true, 1, tc.retryAfter, nil)

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("token_id", 101)
				c.Set("id", 1)
				c.Next()
			})
			router.Use(TokenConcurrentLimit())

			entered := make(chan struct{}, 2)
			release := make(chan struct{})
			var releaseOnce sync.Once
			releaseInFlight := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(releaseInFlight)

			router.POST("/v1/chat/completions", func(c *gin.Context) {
				entered <- struct{}{}
				<-release
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			// 不同模型各自计数：model-a、model-b 可同时在途（各占 1 个槽位）
			blocked := make(chan *httptest.ResponseRecorder, 2)
			for _, modelName := range []string{"model-a", "model-b"} {
				go func(modelName string) {
					recorder := httptest.NewRecorder()
					router.ServeHTTP(recorder, newChatCompletionRequest(modelName))
					blocked <- recorder
				}(modelName)
			}
			<-entered
			<-entered

			rejectedA := httptest.NewRecorder()
			router.ServeHTTP(rejectedA, newChatCompletionRequest("model-a"))
			require.Equal(t, http.StatusTooManyRequests, rejectedA.Code)
			assert.Equal(t, tc.wantHeader, rejectedA.Header().Get("Retry-After"))
			bodyA := decodeRateLimitBody(t, rejectedA)
			assert.Equal(t, "rate_limit_error", bodyA.Error.Type)
			assert.Contains(t, bodyA.Error.Message, "对模型 model-a")
			assert.Contains(t, bodyA.Error.Message, "已达上限（1）")

			rejectedB := httptest.NewRecorder()
			router.ServeHTTP(rejectedB, newChatCompletionRequest("model-b"))
			require.Equal(t, http.StatusTooManyRequests, rejectedB.Code)
			assert.Contains(t, decodeRateLimitBody(t, rejectedB).Error.Message, "对模型 model-b")

			releaseInFlight()
			for i := 0; i < 2; i++ {
				recorder := <-blocked
				require.Equal(t, http.StatusOK, recorder.Code)
			}

			retry := httptest.NewRecorder()
			router.ServeHTTP(retry, newChatCompletionRequest("model-a"))
			require.Equal(t, http.StatusOK, retry.Code)
		})
	}
}

func TestTokenConcurrentLimitModelOverrideRaisesLimit(t *testing.T) {
	withTokenConcurrentLimitSettings(t, true, 1, 5, map[string]int{"model-b": 2})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("token_id", 101)
		c.Set("id", 1)
		c.Next()
	})
	router.Use(TokenConcurrentLimit())

	entered := make(chan struct{}, 3)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseInFlight := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseInFlight)

	router.POST("/v1/chat/completions", func(c *gin.Context) {
		entered <- struct{}{}
		<-release
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// model-b 覆盖为 2：两个请求可同时在途
	blocked := make(chan *httptest.ResponseRecorder, 3)
	for i := 0; i < 2; i++ {
		go func() {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, newChatCompletionRequest("model-b"))
			blocked <- recorder
		}()
	}
	<-entered
	<-entered

	rejectedB := httptest.NewRecorder()
	router.ServeHTTP(rejectedB, newChatCompletionRequest("model-b"))
	require.Equal(t, http.StatusTooManyRequests, rejectedB.Code)
	bodyB := decodeRateLimitBody(t, rejectedB)
	assert.Contains(t, bodyB.Error.Message, "对模型 model-b")
	assert.Contains(t, bodyB.Error.Message, "已达上限（2）")

	// model-a 未覆盖：沿用默认 1
	go func() {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, newChatCompletionRequest("model-a"))
		blocked <- recorder
	}()
	<-entered

	rejectedA := httptest.NewRecorder()
	router.ServeHTTP(rejectedA, newChatCompletionRequest("model-a"))
	require.Equal(t, http.StatusTooManyRequests, rejectedA.Code)
	assert.Contains(t, decodeRateLimitBody(t, rejectedA).Error.Message, "已达上限（1）")

	releaseInFlight()
	for i := 0; i < 3; i++ {
		recorder := <-blocked
		require.Equal(t, http.StatusOK, recorder.Code)
	}
}
