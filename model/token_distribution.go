package model

import (
	"strings"

	"gorm.io/gorm"
)

// token_distribution.go —— token 分布统计查询（MySQL 专用，直查 logs 表）。
//
// 与 rank 查询一样，这些查询依赖 MySQL 专用函数（SUBSTRING_INDEX、CASE WHEN），
// 且 GetRequestCountDistribution 依赖 logs.user_input 列（commit 2 已加）。
// 按用户决策（见 memory: feedback-rank-mysql-only），这是三库兼容规则的明确例外。
//
// 移植自 MIXAPI model/usage_statistics.go 的 GetPromptTokensDistribution 等三个函数。

// DistributionData 分布统计单元
type DistributionData struct {
	TokenName    string  `json:"token_name" gorm:"column:token_name"`
	RangeGroup   string  `json:"range_grp" gorm:"column:range_grp"`
	Count        int64   `json:"count" gorm:"column:count"`
	Percent      float64 `json:"percent" gorm:"-"`
	TotalCount   int64   `json:"total_count" gorm:"-"`
	TotalPercent float64 `json:"total_percent" gorm:"-"`
}

// distributionRangeGroupsPrompt 输入 tokens 分布的桶定义
var distributionRangeGroupsPrompt = []string{
	"0-1k", "1-2k", "2-3k", "3-4k", "4-5k", "5-6k", "6-7k", "7-8k", "8-9k",
	"9-10k", "10-15k", "15-20k", "20-30k", "30-50k", "50-60k", "60-70k",
	"70-80k", "80-90k", "90-100k", ">100k",
}

// distributionRangeGroupsCompletion 输出 tokens 分布的桶定义
var distributionRangeGroupsCompletion = []string{
	"0-256", "256-512", "512-768", "768-1k", "1k-1.5k", "1.5k-2k", "2k-3k",
	"3k-4k", "4k-5k", "5k-6k", "6k-8k", "8k-10k", ">10k",
}

// distributionRangeGroupsRequestCount 单问题请求次数分布的桶定义
var distributionRangeGroupsRequestCount = []string{
	"1-2", "3-5", "6-10", "11-20", "21-30", "31-40", "41-50", "51-60",
	"61-70", "71-80", "81-90", "91-100", ">100",
}

// distributionExcludeTokenNames 排除的 token 名称（playground/测试）
var distributionExcludeTokenNames = []string{"playground-default", "模型测试"}

// applyDistributionDateRange 给 logs 查询附加 created_at 日期范围条件。
func applyDistributionDateRange(query *gorm.DB, startDate, endDate string) *gorm.DB {
	if startDate != "" {
		ts := parseTimestamp(dateToTimestamp(startDate))
		if ts > 0 {
			query = query.Where("created_at >= ?", ts)
		}
	}
	if endDate != "" {
		endDateStr := endDate
		if len(endDate) >= 10 {
			endDateStr = endDate[0:10] + " 23:59:59"
		} else {
			endDateStr = endDate + " 23:59:59"
		}
		ts := parseTimestamp(endDateStr)
		if ts > 0 {
			query = query.Where("created_at <= ?", ts)
		}
	}
	return query
}

// applyDistributionCommonFilters 给 logs 查询附加 user_id/token_ids/model_name/排除名等公共条件。
func applyDistributionCommonFilters(query *gorm.DB, tokenIds, modelName string, userId int) *gorm.DB {
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	if tokenIds != "" {
		query = query.Where("token_id IN ?", strings.Split(tokenIds, ","))
	}
	if modelName != "" {
		query = query.Where("model_name LIKE ?", "%"+modelName+"%")
	}
	query = query.Where("token_name NOT IN (?)", distributionExcludeTokenNames)
	return query
}

// distributionGroupExpr 返回分组表达式（exact 用 token_name，prefix 用 SUBSTRING_INDEX）
func distributionGroupExpr(groupBy string) string {
	if groupBy == "prefix" {
		return "SUBSTRING_INDEX(token_name, '-', 1)"
	}
	return "token_name"
}

// completeAndPageDistribution 补全分布矩阵（缺失桶补 0）并按 token 名分页。
// 返回分页后的分布数据、桶列表、token 总数。
func completeAndPageDistribution(distributions []*DistributionData, rangeGroups []string, page, pageSize int) ([]*DistributionData, []string, int64) {
	tokenTotals := make(map[string]int64)
	var grandTotal int64
	for _, d := range distributions {
		tokenTotals[d.TokenName] += d.Count
		grandTotal += d.Count
	}

	for _, d := range distributions {
		if total, ok := tokenTotals[d.TokenName]; ok && total > 0 {
			d.Percent = float64(d.Count) / float64(total) * 100
		}
		d.TotalCount = grandTotal
		if grandTotal > 0 {
			d.TotalPercent = float64(d.Count) / float64(grandTotal) * 100
		}
	}

	tokenNames := make([]string, 0, len(tokenTotals))
	for name := range tokenTotals {
		tokenNames = append(tokenNames, name)
	}

	completeDistributions := make([]*DistributionData, 0, len(distributions)+len(tokenNames)*len(rangeGroups))
	existingKey := make(map[string]bool, len(distributions))
	for _, d := range distributions {
		key := d.TokenName + "|" + d.RangeGroup
		existingKey[key] = true
		completeDistributions = append(completeDistributions, d)
	}
	for _, tokenName := range tokenNames {
		for _, rangeGroup := range rangeGroups {
			key := tokenName + "|" + rangeGroup
			if !existingKey[key] {
				completeDistributions = append(completeDistributions, &DistributionData{
					TokenName:  tokenName,
					RangeGroup: rangeGroup,
					Count:      0,
					Percent:    0,
				})
			}
		}
	}

	total := int64(len(tokenNames))
	offset := (page - 1) * pageSize
	if offset > len(tokenNames) {
		offset = len(tokenNames)
	}
	endOffset := offset + pageSize
	if endOffset > len(tokenNames) {
		endOffset = len(tokenNames)
	}

	pageTokenNames := tokenNames[offset:endOffset]
	pageTokenSet := make(map[string]bool, len(pageTokenNames))
	for _, name := range pageTokenNames {
		pageTokenSet[name] = true
	}

	pagedDistributions := make([]*DistributionData, 0)
	for _, d := range completeDistributions {
		if pageTokenSet[d.TokenName] {
			pagedDistributions = append(pagedDistributions, d)
		}
	}

	return pagedDistributions, rangeGroups, total
}

// GetPromptTokensDistribution 输入 tokens 分布统计
func GetPromptTokensDistribution(startDate, endDate string, tokenIds string, modelName string, userId int, groupBy string, page, pageSize int) ([]*DistributionData, []string, int64, error) {
	var distributions []*DistributionData
	groupByExpr := distributionGroupExpr(groupBy)

	query := LOG_DB.Table("logs").Select(`
		` + groupByExpr + ` as token_name,
		CASE
			WHEN prompt_tokens < 1000 THEN '0-1k'
			WHEN prompt_tokens < 2000 THEN '1-2k'
			WHEN prompt_tokens < 3000 THEN '2-3k'
			WHEN prompt_tokens < 4000 THEN '3-4k'
			WHEN prompt_tokens < 5000 THEN '4-5k'
			WHEN prompt_tokens < 6000 THEN '5-6k'
			WHEN prompt_tokens < 7000 THEN '6-7k'
			WHEN prompt_tokens < 8000 THEN '7-8k'
			WHEN prompt_tokens < 9000 THEN '8-9k'
			WHEN prompt_tokens < 10000 THEN '9-10k'
			WHEN prompt_tokens < 15000 THEN '10-15k'
			WHEN prompt_tokens < 20000 THEN '15-20k'
			WHEN prompt_tokens < 30000 THEN '20-30k'
			WHEN prompt_tokens < 50000 THEN '30-50k'
			WHEN prompt_tokens < 60000 THEN '50-60k'
			WHEN prompt_tokens < 70000 THEN '60-70k'
			WHEN prompt_tokens < 80000 THEN '70-80k'
			WHEN prompt_tokens < 90000 THEN '80-90k'
			WHEN prompt_tokens < 100000 THEN '90-100k'
			ELSE '>100k'
		END AS range_grp,
		COUNT(*) AS count
	`).Where("prompt_tokens > 0")

	query = applyDistributionCommonFilters(query, tokenIds, modelName, userId)
	query = applyDistributionDateRange(query, startDate, endDate)

	err := query.Group(groupByExpr + ", range_grp").Order("token_name, MIN(prompt_tokens)").Scan(&distributions).Error
	if err != nil {
		return nil, []string{}, 0, err
	}

	paged, groups, total := completeAndPageDistribution(distributions, distributionRangeGroupsPrompt, page, pageSize)
	return paged, groups, total, nil
}

// GetCompletionTokensDistribution 输出 tokens 分布统计
func GetCompletionTokensDistribution(startDate, endDate string, tokenIds string, modelName string, userId int, groupBy string, page, pageSize int) ([]*DistributionData, []string, int64, error) {
	var distributions []*DistributionData
	groupByExpr := distributionGroupExpr(groupBy)

	query := LOG_DB.Table("logs").Select(`
		` + groupByExpr + ` as token_name,
		CASE
			WHEN completion_tokens < 256 THEN '0-256'
			WHEN completion_tokens < 512 THEN '256-512'
			WHEN completion_tokens < 768 THEN '512-768'
			WHEN completion_tokens < 1024 THEN '768-1k'
			WHEN completion_tokens < 1536 THEN '1k-1.5k'
			WHEN completion_tokens < 2048 THEN '1.5k-2k'
			WHEN completion_tokens < 3072 THEN '2k-3k'
			WHEN completion_tokens < 4096 THEN '3k-4k'
			WHEN completion_tokens < 5120 THEN '4k-5k'
			WHEN completion_tokens < 6144 THEN '5k-6k'
			WHEN completion_tokens < 8192 THEN '6k-8k'
			WHEN completion_tokens < 10240 THEN '8k-10k'
			ELSE '>10k'
		END AS range_grp,
		COUNT(*) AS count
	`).Where("completion_tokens > 0")

	query = applyDistributionCommonFilters(query, tokenIds, modelName, userId)
	query = applyDistributionDateRange(query, startDate, endDate)

	err := query.Group(groupByExpr + ", range_grp").Order("token_name, MIN(completion_tokens)").Scan(&distributions).Error
	if err != nil {
		return nil, []string{}, 0, err
	}

	paged, groups, total := completeAndPageDistribution(distributions, distributionRangeGroupsCompletion, page, pageSize)
	return paged, groups, total, nil
}

// GetRequestCountDistribution 单问题请求次数分布统计（依赖 logs.user_input）
func GetRequestCountDistribution(startDate, endDate string, tokenIds string, modelName string, userId int, groupBy string, page, pageSize int) ([]*DistributionData, []string, int64, error) {
	var distributions []*DistributionData
	groupByExpr := distributionGroupExpr(groupBy)

	subQuery := LOG_DB.Table("logs").Select(`
		` + groupByExpr + ` as token_name,
		user_input,
		COUNT(*) as call_count
	`).Where("user_input IS NOT NULL AND user_input != '' AND user_input != '[]' AND user_input != 'null' AND LENGTH(user_input) > 2")

	subQuery = applyDistributionCommonFilters(subQuery, tokenIds, modelName, userId)
	subQuery = applyDistributionDateRange(subQuery, startDate, endDate)
	subQuery = subQuery.Group(groupByExpr + ", user_input")

	query := LOG_DB.Table("(?) as input_stats", subQuery).Select(`
		token_name,
		CASE
			WHEN call_count BETWEEN 1 AND 2 THEN '1-2'
			WHEN call_count BETWEEN 3 AND 5 THEN '3-5'
			WHEN call_count BETWEEN 6 AND 10 THEN '6-10'
			WHEN call_count BETWEEN 11 AND 20 THEN '11-20'
			WHEN call_count BETWEEN 21 AND 30 THEN '21-30'
			WHEN call_count BETWEEN 31 AND 40 THEN '31-40'
			WHEN call_count BETWEEN 41 AND 50 THEN '41-50'
			WHEN call_count BETWEEN 51 AND 60 THEN '51-60'
			WHEN call_count BETWEEN 61 AND 70 THEN '61-70'
			WHEN call_count BETWEEN 71 AND 80 THEN '71-80'
			WHEN call_count BETWEEN 81 AND 90 THEN '81-90'
			WHEN call_count BETWEEN 91 AND 100 THEN '91-100'
			ELSE '>100'
		END AS range_grp,
		COUNT(*) AS count
	`).Group("token_name, range_grp")

	err := query.Order("token_name, MIN(call_count)").Scan(&distributions).Error
	if err != nil {
		return nil, []string{}, 0, err
	}

	paged, groups, total := completeAndPageDistribution(distributions, distributionRangeGroupsRequestCount, page, pageSize)
	return paged, groups, total, nil
}
