package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// token_distribution.go —— token 分布统计 controller（prompt/completion/request-count）。
// 移植自 MIXAPI controller/token_distribution.go，改用 common.ApiSuccess 风格，导出用 CSV。
// 对应 model 函数为 MySQL 专用（见 model/token_distribution.go）。

// defaultMonthStartRange 返回本月1号到今天的日期范围。
func defaultMonthStartRange() (string, string) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
	end := now.Format("2006-01-02")
	return start, end
}

func distributionParams(c *gin.Context) (string, string, string, string, string, int, int) {
	page, pageSize := parseUsagePage(c)
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	tokenIds := c.Query("token_ids")
	modelName := c.Query("model_name")
	groupBy := c.DefaultQuery("group_by", "prefix")
	if startDate == "" && endDate == "" {
		startDate, endDate = defaultMonthStartRange()
	}
	return startDate, endDate, tokenIds, modelName, groupBy, page, pageSize
}

// GetPromptTokensDistribution 输入 tokens 分布（管理员）
func GetPromptTokensDistribution(c *gin.Context) {
	startDate, endDate, tokenIds, modelName, groupBy, page, pageSize := distributionParams(c)
	distributions, rangeGroups, total, err := model.GetPromptTokensDistribution(startDate, endDate, tokenIds, modelName, 0, groupBy, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":        distributions,
		"range_groups": rangeGroups,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
	})
}

// GetUserPromptTokensDistribution 输入 tokens 分布（用户）
func GetUserPromptTokensDistribution(c *gin.Context) {
	userId := c.GetInt("id")
	startDate, endDate, tokenIds, modelName, groupBy, page, pageSize := distributionParams(c)
	distributions, rangeGroups, total, err := model.GetPromptTokensDistribution(startDate, endDate, tokenIds, modelName, userId, groupBy, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":        distributions,
		"range_groups": rangeGroups,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
	})
}

// GetCompletionTokensDistribution 输出 tokens 分布（管理员）
func GetCompletionTokensDistribution(c *gin.Context) {
	startDate, endDate, tokenIds, modelName, groupBy, page, pageSize := distributionParams(c)
	distributions, rangeGroups, total, err := model.GetCompletionTokensDistribution(startDate, endDate, tokenIds, modelName, 0, groupBy, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":        distributions,
		"range_groups": rangeGroups,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
	})
}

// GetUserCompletionTokensDistribution 输出 tokens 分布（用户）
func GetUserCompletionTokensDistribution(c *gin.Context) {
	userId := c.GetInt("id")
	startDate, endDate, tokenIds, modelName, groupBy, page, pageSize := distributionParams(c)
	distributions, rangeGroups, total, err := model.GetCompletionTokensDistribution(startDate, endDate, tokenIds, modelName, userId, groupBy, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":        distributions,
		"range_groups": rangeGroups,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
	})
}

// GetRequestCountDistribution 单问题请求次数分布（管理员）
func GetRequestCountDistribution(c *gin.Context) {
	startDate, endDate, tokenIds, modelName, groupBy, page, pageSize := distributionParams(c)
	distributions, rangeGroups, total, err := model.GetRequestCountDistribution(startDate, endDate, tokenIds, modelName, 0, groupBy, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":        distributions,
		"range_groups": rangeGroups,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
	})
}

// GetUserRequestCountDistribution 单问题请求次数分布（用户）
func GetUserRequestCountDistribution(c *gin.Context) {
	userId := c.GetInt("id")
	startDate, endDate, tokenIds, modelName, groupBy, page, pageSize := distributionParams(c)
	distributions, rangeGroups, total, err := model.GetRequestCountDistribution(startDate, endDate, tokenIds, modelName, userId, groupBy, page, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items":        distributions,
		"range_groups": rangeGroups,
		"total":        total,
		"page":         page,
		"page_size":    pageSize,
	})
}

// ExportPromptTokensDistribution 导出输入 tokens 分布为 CSV
func ExportPromptTokensDistribution(c *gin.Context) {
	exportDistribution(c, "prompt")
}

// ExportCompletionTokensDistribution 导出输出 tokens 分布为 CSV
func ExportCompletionTokensDistribution(c *gin.Context) {
	exportDistribution(c, "completion")
}

// ExportRequestCountDistribution 导出单问题请求次数分布为 CSV
func ExportRequestCountDistribution(c *gin.Context) {
	exportDistribution(c, "request_count")
}

// exportDistribution 通用分布导出：kind 决定调用哪个 model 函数与桶列表。
func exportDistribution(c *gin.Context, kind string) {
	userId := c.GetInt("id")
	startDate, endDate, tokenIds, modelName, groupBy, _, _ := distributionParams(c)

	var distributions []*model.DistributionData
	var rangeGroups []string
	var err error
	if userId > 0 && c.Query("admin") == "" {
		switch kind {
		case "prompt":
			distributions, rangeGroups, _, err = model.GetPromptTokensDistribution(startDate, endDate, tokenIds, modelName, userId, groupBy, 1, 10000)
		case "completion":
			distributions, rangeGroups, _, err = model.GetCompletionTokensDistribution(startDate, endDate, tokenIds, modelName, userId, groupBy, 1, 10000)
		case "request_count":
			distributions, rangeGroups, _, err = model.GetRequestCountDistribution(startDate, endDate, tokenIds, modelName, userId, groupBy, 1, 10000)
		}
	} else {
		switch kind {
		case "prompt":
			distributions, rangeGroups, _, err = model.GetPromptTokensDistribution(startDate, endDate, tokenIds, modelName, 0, groupBy, 1, 10000)
		case "completion":
			distributions, rangeGroups, _, err = model.GetCompletionTokensDistribution(startDate, endDate, tokenIds, modelName, 0, groupBy, 1, 10000)
		case "request_count":
			distributions, rangeGroups, _, err = model.GetRequestCountDistribution(startDate, endDate, tokenIds, modelName, 0, groupBy, 1, 10000)
		}
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}

	buf := make([]byte, 0, 4096)
	buf = append(buf, []byte("\xEF\xBB\xBF")...)
	header := append([]string{"令牌名称"}, rangeGroups...)
	header = append(header, "总计")
	buf = appendCSVRow(buf, header)

	// 按 token 聚合每个桶的 count
	tokens := make(map[string]map[string]int64)
	tokenOrder := make([]string, 0)
	for _, d := range distributions {
		if _, ok := tokens[d.TokenName]; !ok {
			tokens[d.TokenName] = make(map[string]int64)
			tokenOrder = append(tokenOrder, d.TokenName)
		}
		tokens[d.TokenName][d.RangeGroup] += d.Count
	}

	for _, name := range tokenOrder {
		row := []string{name}
		var sum int64
		for _, g := range rangeGroups {
			cnt := tokens[name][g]
			row = append(row, strconv.FormatInt(cnt, 10))
			sum += cnt
		}
		row = append(row, strconv.FormatInt(sum, 10))
		buf = appendCSVRow(buf, row)
	}

	filename := "distribution_" + kind + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf)
}
