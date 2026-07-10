package model

import (
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
)

// extractUserInputFromContext 从 gin.Context 中取出 Claude 请求并提取用户输入。
// 受 common.LogUserInputEnabled 开关控制；非 Claude 请求或开关关闭时返回空串。
func extractUserInputFromContext(c *gin.Context) string {
	if c == nil || !common.LogUserInputEnabled {
		return ""
	}
	raw, exists := c.Get("claude_request")
	if !exists || raw == nil {
		return ""
	}
	claudeRequest, ok := raw.(*dto.ClaudeRequest)
	if !ok || claudeRequest == nil {
		return ""
	}
	return extractUserInputFromClaudeMessages(claudeRequest.Messages)
}

// extractUserInputFromClaudeMessages 从 Claude 消息列表中提取最后一条有效的用户输入内容。
// 仅保留汉字与标点，过滤代码类内容。对应 MIXAPI 的同名逻辑，原样移植。
func extractUserInputFromClaudeMessages(messages []dto.ClaudeMessage) string {
	if len(messages) == 0 {
		return ""
	}

	// 从后往前查找，只提取最后一条用户消息
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message.Role == "user" {
			content := message.GetStringContent()
			if content != "" {
				if isClaudeCodeContent(content) {
					continue
				}
				filteredContent := filterClaudeChineseContent(content)
				if filteredContent != "" {
					return filteredContent
				}
			}
		}
	}

	// 没有找到合适的用户消息时，回退到最后一条消息内容
	lastContent := messages[len(messages)-1].GetStringContent()
	if isClaudeCodeContent(lastContent) {
		return ""
	}
	return filterClaudeChineseContent(lastContent)
}

// isClaudeCodeContent 判断内容是否为代码类（Claude Code 注入的上下文等）
func isClaudeCodeContent(content string) bool {
	codeKeywords := []string{
		"VSCode Open Tabs",
		"Current Time",
		"Current Cost",
		"Current Mode",
		"REMINDERS",
		"VSCode Visible Files",
	}
	for _, keyword := range codeKeywords {
		if strings.Contains(content, keyword) {
			return true
		}
	}
	return false
}

// filterClaudeChineseContent 只保留汉字与标点字符
func filterClaudeChineseContent(content string) string {
	var result []rune
	for _, r := range content {
		if unicode.Is(unicode.Scripts["Han"], r) || unicode.IsPunct(r) {
			result = append(result, r)
		}
	}
	return string(result)
}
