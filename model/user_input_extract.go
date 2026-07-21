package model

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
)

// extractUserInputFromContext 从 gin.Context 中取出已校验的请求并提取最后一条用户输入内容。
// 受 common.LogUserInputEnabled 开关控制；开关关闭或无法识别请求时返回空串。
// 优先读取 Claude 请求（原生 Claude 与 OpenAI→Claude 转换路径都会注入 claude_request），
// 其次读取通用 relay_request（OpenAI/Gemini/Responses 等原生路径注入），覆盖所有协议。
func extractUserInputFromContext(c *gin.Context) string {
	if c == nil || !common.LogUserInputEnabled {
		return ""
	}
	if raw, exists := c.Get("claude_request"); exists && raw != nil {
		if claudeRequest, ok := raw.(*dto.ClaudeRequest); ok && claudeRequest != nil {
			if content := extractUserInputFromClaudeMessages(claudeRequest.Messages); content != "" {
				return content
			}
		}
	}
	if raw, exists := c.Get("relay_request"); exists && raw != nil {
		if openAIRequest, ok := raw.(*dto.GeneralOpenAIRequest); ok && openAIRequest != nil {
			return extractUserInputFromOpenAIMessages(openAIRequest.Messages)
		}
	}
	return ""
}

// extractUserInputFromOpenAIMessages 从 OpenAI 消息列表中提取最后一条用户输入的文本内容。
func extractUserInputFromOpenAIMessages(messages []dto.Message) string {
	if len(messages) == 0 {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			if content := messages[i].StringContent(); content != "" {
				return content
			}
		}
	}
	return ""
}

// extractUserInputFromClaudeMessages 从 Claude 消息列表中提取最后一条用户输入内容（原文，不过滤）。
func extractUserInputFromClaudeMessages(messages []dto.ClaudeMessage) string {
	if len(messages) == 0 {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			if content := messages[i].GetStringContent(); content != "" {
				return content
			}
		}
	}
	return ""
}
