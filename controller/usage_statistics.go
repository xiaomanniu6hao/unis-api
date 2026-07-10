package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// usage_statistics.go —— 用量统计 controller（日/月/rank + summary + 导出）。
// 移植自 MIXAPI controller/usage_statistics.go 与 usage_statistics_rank.go，
// 改用 new-api-dev 的 common.ApiSuccess/ApiError 风格，导出用 CSV（无新依赖）。

// defaultDateRangeDays 返回最近 n 天的起止日期（YYYY-MM-DD）。
func defaultDateRangeDays(days int) (string, string) {
	now := time.Now()
	start := now.AddDate(0, 0, -days).Format("2006-01-02")
	end := now.Format("2006-01-02")
	return start, end
}

// defaultMonthRange 返回最近 months 个月的起止月份（YYYY-MM）。
func defaultMonthRange(months int) (string, string) {
	now := time.Now()
	start := now.AddDate(0, -months, 0).Format("2006-01")
	end := now.Format("2006-01")
	return start, end
}

func parseUsagePage(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}

// GetUsageStatistics 获取用量统计数据（按日，管理员）
func GetUsageStatistics(c *gin.Context) {
	page, pageSize := parseUsagePage(c)
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	tokenId, _ := strconv.Atoi(c.Query("token_id"))
	modelName := c.Query("model_name")

	if startDate == "" && endDate == "" {
		startDate, endDate = defaultDateRangeDays(7)
	}

	statistics, total, err := model.GetUsageStatistics(startDate, endDate, tokenId, modelName, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	summary, err := model.GetUsageStatisticsSummary(startDate, endDate, tokenId, modelName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":     statistics,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"summary":   summary,
	})
}

// GetUsageStatisticsSummary 获取用量统计摘要（按日，管理员）
func GetUsageStatisticsSummary(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	tokenId, _ := strconv.Atoi(c.Query("token_id"))
	modelName := c.Query("model_name")
	if startDate == "" && endDate == "" {
		startDate, endDate = defaultDateRangeDays(7)
	}
	summary, err := model.GetUsageStatisticsSummary(startDate, endDate, tokenId, modelName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

// GetUserUsageStatistics 获取用户用量统计数据（按日）
func GetUserUsageStatistics(c *gin.Context) {
	userId := c.GetInt("id")
	page, pageSize := parseUsagePage(c)
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	tokenId, _ := strconv.Atoi(c.Query("token_id"))
	modelName := c.Query("model_name")
	if startDate == "" && endDate == "" {
		startDate, endDate = defaultDateRangeDays(7)
	}
	statistics, total, err := model.GetUserUsageStatistics(userId, startDate, endDate, tokenId, modelName, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	summary, err := model.GetUserUsageStatisticsSummary(userId, startDate, endDate, tokenId, modelName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":     statistics,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"summary":   summary,
	})
}

// GetMonthlyUsageStatistics 获取月度用量统计数据（管理员）
func GetMonthlyUsageStatistics(c *gin.Context) {
	page, pageSize := parseUsagePage(c)
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	tokenId, _ := strconv.Atoi(c.Query("token_id"))
	modelName := c.Query("model_name")
	if startDate == "" && endDate == "" {
		startDate, endDate = defaultMonthRange(6)
	}
	statistics, total, err := model.GetMonthlyUsageStatistics(startDate, endDate, tokenId, modelName, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	summary, err := model.GetMonthlyUsageStatisticsSummary(startDate, endDate, tokenId, modelName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":     statistics,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"summary":   summary,
	})
}

// GetMonthlyUsageStatisticsSummary 获取月度用量统计摘要（管理员）
func GetMonthlyUsageStatisticsSummary(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	tokenId, _ := strconv.Atoi(c.Query("token_id"))
	modelName := c.Query("model_name")
	if startDate == "" && endDate == "" {
		startDate, endDate = defaultMonthRange(6)
	}
	summary, err := model.GetMonthlyUsageStatisticsSummary(startDate, endDate, tokenId, modelName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

// GetUserMonthlyUsageStatistics 获取用户月度用量统计数据
func GetUserMonthlyUsageStatistics(c *gin.Context) {
	userId := c.GetInt("id")
	page, pageSize := parseUsagePage(c)
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	tokenId, _ := strconv.Atoi(c.Query("token_id"))
	modelName := c.Query("model_name")
	if startDate == "" && endDate == "" {
		startDate, endDate = defaultMonthRange(6)
	}
	statistics, total, err := model.GetUserMonthlyUsageStatistics(userId, startDate, endDate, tokenId, modelName, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	summary, err := model.GetUserMonthlyUsageStatisticsSummary(userId, startDate, endDate, tokenId, modelName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":     statistics,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"summary":   summary,
	})
}

// GetRankUsageStatistics 获取用量排序统计数据（管理员）
func GetRankUsageStatistics(c *gin.Context) {
	page, pageSize := parseUsagePage(c)
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	tokenIds := c.Query("token_ids")
	modelName := c.Query("model_name")
	groupBy := c.DefaultQuery("group_by", "prefix")
	if startDate == "" && endDate == "" {
		startDate, endDate = defaultMonthRange(6)
	}
	statistics, total, err := model.GetRankUsageStatistics(startDate, endDate, tokenIds, modelName, groupBy, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	summary, err := model.GetRankUsageStatisticsSummary(startDate, endDate, tokenIds, modelName, groupBy)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":     statistics,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"summary":   summary,
	})
}

// GetUserRankUsageStatistics 获取用户用量排序统计数据
func GetUserRankUsageStatistics(c *gin.Context) {
	userId := c.GetInt("id")
	page, pageSize := parseUsagePage(c)
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	tokenIds := c.Query("token_ids")
	modelName := c.Query("model_name")
	groupBy := c.DefaultQuery("group_by", "prefix")
	if startDate == "" && endDate == "" {
		startDate, endDate = defaultMonthRange(6)
	}
	statistics, total, err := model.GetUserRankUsageStatistics(userId, startDate, endDate, tokenIds, modelName, groupBy, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	summary, err := model.GetUserRankUsageStatisticsSummary(userId, startDate, endDate, tokenIds, modelName, groupBy)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":     statistics,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"summary":   summary,
	})
}

// rankCSVHeaders 是 rank 导出的 CSV 表头
var rankCSVHeaders = []string{
	"令牌名称", "总请求数", "成功请求", "失败请求", "总Tokens",
	"输入Tokens", "输出Tokens", "缓存Tokens", "总消费额度",
	"平均输入Tokens", "最小输入Tokens", "最大输入Tokens",
	"平均输出Tokens", "最小输出Tokens", "最大输出Tokens",
	"问题数", "平均每问题请求数", "最小每问题请求数", "最大每问题请求数",
}

// rankCSVRow 把一条统计记录转成 CSV 行
func rankCSVRow(s *model.UsageStatistics) []string {
	return []string{
		s.TokenName,
		strconv.Itoa(s.TotalRequests),
		strconv.Itoa(s.SuccessfulRequests),
		strconv.Itoa(s.FailedRequests),
		strconv.Itoa(s.TotalTokens),
		strconv.Itoa(s.PromptTokens),
		strconv.Itoa(s.CompletionTokens),
		strconv.Itoa(s.PromptTokensCache),
		strconv.Itoa(s.TotalQuota),
		strconv.FormatFloat(s.AvgPromptTokens, 'f', 2, 64),
		strconv.Itoa(s.MinPromptTokens),
		strconv.Itoa(s.MaxPromptTokens),
		strconv.FormatFloat(s.AvgCompletionTokens, 'f', 2, 64),
		strconv.Itoa(s.MinCompletionTokens),
		strconv.Itoa(s.MaxCompletionTokens),
		strconv.Itoa(s.QuestionCount),
		strconv.FormatFloat(s.AvgRequestsPerQuestion, 'f', 2, 64),
		strconv.Itoa(s.MinRequestsPerQuestion),
		strconv.Itoa(s.MaxRequestsPerQuestion),
	}
}

// ExportRankUsageStatistics 导出用量排序统计为 CSV
func ExportRankUsageStatistics(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	tokenIds := c.Query("token_ids")
	modelName := c.Query("model_name")
	groupBy := c.DefaultQuery("group_by", "prefix")
	userId := c.GetInt("id")
	if startDate == "" && endDate == "" {
		startDate, endDate = defaultMonthRange(6)
	}

	var statistics []*model.UsageStatistics
	var err error
	if userId > 0 && c.Query("admin") == "" {
		statistics, _, err = model.GetUserRankUsageStatistics(userId, startDate, endDate, tokenIds, modelName, groupBy, 1, 10000)
	} else {
		statistics, _, err = model.GetRankUsageStatistics(startDate, endDate, tokenIds, modelName, groupBy, 1, 10000)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 写 CSV（带 UTF-8 BOM 以便 Excel 正确识别中文）
	buf := make([]byte, 0, 4096)
	buf = append(buf, []byte("\xEF\xBB\xBF")...)
	buf = appendCSVRow(buf, rankCSVHeaders)
	for _, s := range statistics {
		buf = appendCSVRow(buf, rankCSVRow(s))
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=usage_rank.csv")
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf)
}

// appendCSVRow 把一行字段追加到 CSV 缓冲，按 RFC 4180 转义含逗号/引号/换行的字段。
func appendCSVRow(buf []byte, fields []string) []byte {
	for i, f := range fields {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = appendCSVField(buf, f)
	}
	return append(buf, '\n')
}

// appendCSVField 转义单个 CSV 字段
func appendCSVField(buf []byte, f string) []byte {
	needQuote := false
	for _, r := range f {
		if r == ',' || r == '"' || r == '\n' || r == '\r' {
			needQuote = true
			break
		}
	}
	if !needQuote {
		return append(buf, f...)
	}
	buf = append(buf, '"')
	for _, r := range f {
		if r == '"' {
			buf = append(buf, '"', '"')
		} else {
			buf = append(buf, string(r)...)
		}
	}
	return append(buf, '"')
}
