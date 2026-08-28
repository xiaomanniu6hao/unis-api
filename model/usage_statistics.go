package model

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// parseTimestamp 解析 "2006-01-02 15:04:05" 格式的时间字符串为 Unix 时间戳
func parseTimestamp(dateStr string) int64 {
	t, err := time.Parse("2006-01-02 15:04:05", dateStr)
	if err != nil {
		return 0
	}
	return t.Unix()
}

// dateToTimestamp 把 "YYYY-MM-DD" 归一化为 "YYYY-MM-DD 00:00:00" 再转时间戳
func dateToTimestamp(dateStr string) string {
	if len(dateStr) >= 10 {
		return dateStr[0:10] + " 00:00:00"
	}
	return dateStr + " 00:00:00"
}

// UsageStatistics 用量统计日汇总表，按 (date, token_id, model_name) 唯一
type UsageStatistics struct {
	Id                     int     `json:"id" gorm:"primaryKey"`
	Date                   string  `json:"date" gorm:"type:varchar(10);not null;index:idx_date;uniqueIndex:uk_date_token_model,composite:date"`
	TokenId                int     `json:"token_id" gorm:"not null;index:idx_token_id;uniqueIndex:uk_date_token_model,composite:token_id"`
	TokenName              string  `json:"token_name" gorm:"type:varchar(255);not null;default:''"`
	ModelName              string  `json:"model_name" gorm:"type:varchar(255);not null;index:idx_model_name;uniqueIndex:uk_date_token_model,composite:model_name"`
	TotalRequests          int     `json:"total_requests" gorm:"not null;default:0"`
	SuccessfulRequests     int     `json:"successful_requests" gorm:"not null;default:0"`
	FailedRequests         int     `json:"failed_requests" gorm:"not null;default:0"`
	TotalTokens            int     `json:"total_tokens" gorm:"not null;default:0"`
	PromptTokens           int     `json:"prompt_tokens" gorm:"not null;default:0"`
	CompletionTokens       int     `json:"completion_tokens" gorm:"not null;default:0"`
	PromptTokensCache      int     `json:"prompt_tokens_cache" gorm:"not null;default:0"`
	TotalQuota             int     `json:"total_quota" gorm:"not null;default:0"`
	AvgPromptTokens        float64 `json:"avg_prompt_tokens"`
	MinPromptTokens        int     `json:"min_prompt_tokens"`
	MaxPromptTokens        int     `json:"max_prompt_tokens"`
	AvgCompletionTokens    float64 `json:"avg_completion_tokens"`
	MinCompletionTokens    int     `json:"min_completion_tokens"`
	MaxCompletionTokens    int     `json:"max_completion_tokens"`
	QuestionCount          int     `json:"question_count" gorm:"not null;default:0"`
	AvgRequestsPerQuestion float64 `json:"avg_requests_per_question"`
	MinRequestsPerQuestion int     `json:"min_requests_per_question"`
	MaxRequestsPerQuestion int     `json:"max_requests_per_question"`
	CreatedTime            int64   `json:"created_time" gorm:"bigint;not null"`
	UpdatedTime            int64   `json:"updated_time" gorm:"bigint;not null"`
}

func (UsageStatistics) TableName() string {
	return "usage_statistics"
}

// GetUsageStatistics 获取用量统计数据（按日）
func GetUsageStatistics(startDate, endDate string, tokenId int, modelName string, page, pageSize int) ([]*UsageStatistics, int64, error) {
	var statistics []*UsageStatistics
	var total int64

	query := DB.Model(&UsageStatistics{})

	if startDate != "" {
		query = query.Where("date >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("date <= ?", endDate)
	}
	if tokenId > 0 {
		query = query.Where("token_id = ?", tokenId)
	}
	if modelName != "" {
		query = query.Where("model_name LIKE ?", "%"+modelName+"%")
	}
	query = applyStatsExclusionGorm(query, "")

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err = query.Order("date DESC, token_id ASC, model_name ASC").
		Offset(offset).Limit(pageSize).Find(&statistics).Error

	return statistics, total, err
}

// GetMonthlyUsageStatistics 获取月度用量统计数据（基于日表按月聚合）
func GetMonthlyUsageStatistics(startDate, endDate string, tokenId int, modelName string, page, pageSize int) ([]*UsageStatistics, int64, error) {
	var statistics []*UsageStatistics
	var total int64

	db := DB.Model(&UsageStatistics{})

	conditions := ""
	params := []interface{}{}

	if startDate != "" {
		conditions += " AND date >= ?"
		params = append(params, startDate+"-01")
	}
	if endDate != "" {
		if len(endDate) >= 7 {
			year := endDate[0:4]
			month := endDate[5:7]
			conditions += " AND date <= ?"
			params = append(params, year+"-"+month+"-31")
		}
	}
	if tokenId > 0 {
		conditions += " AND token_id = ?"
		params = append(params, tokenId)
	}
	if modelName != "" {
		conditions += " AND model_name LIKE ?"
		params = append(params, "%"+modelName+"%")
	}
	conditions, params = appendStatsExclusionCondition(conditions, "token_id", params)

	sql := `
		SELECT
			MAX(id) as id,
			SUBSTR(date, 1, 7) as date,
			token_id,
			token_name,
			model_name,
			SUM(total_requests) as total_requests,
			SUM(successful_requests) as successful_requests,
			SUM(failed_requests) as failed_requests,
			SUM(total_tokens) as total_tokens,
			SUM(prompt_tokens) as prompt_tokens,
			SUM(completion_tokens) as completion_tokens,
			SUM(prompt_tokens_cache) as prompt_tokens_cache,
			SUM(total_quota) as total_quota,
			MAX(created_time) as created_time,
			MAX(updated_time) as updated_time
		FROM usage_statistics
		WHERE 1=1` + conditions + `
		GROUP BY SUBSTR(date, 1, 7), token_id, token_name, model_name
		ORDER BY date DESC, token_id ASC, model_name ASC
	`

	countSQL := `
		SELECT COUNT(*) as count FROM (
			SELECT 1
			FROM usage_statistics
			WHERE 1=1` + conditions + `
			GROUP BY SUBSTR(date, 1, 7), token_id, token_name, model_name
		) as grouped_data
	`

	var countResult struct {
		Count int64 `json:"count"`
	}
	err := db.Raw(countSQL, params...).Scan(&countResult).Error
	if err != nil {
		return nil, 0, err
	}
	total = countResult.Count

	offset := (page - 1) * pageSize
	limitSQL := sql + " LIMIT ? OFFSET ?"
	params = append(params, pageSize, offset)

	err = db.Raw(limitSQL, params...).Scan(&statistics).Error
	return statistics, total, err
}

// UpsertUsageStatistics 插入或更新用量统计数据，兼容 MySQL/PostgreSQL/SQLite
func UpsertUsageStatistics(date string, tokenId int, tokenName, modelName string,
	totalRequests, successfulRequests, failedRequests int,
	totalTokens, promptTokens, completionTokens, cacheTokens, totalQuota int) error {

	now := common.GetTimestamp()

	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		return DB.Exec(`
			INSERT INTO usage_statistics
			(date, token_id, token_name, model_name, total_requests, successful_requests, failed_requests,
			 total_tokens, prompt_tokens, completion_tokens, prompt_tokens_cache, total_quota, created_time, updated_time)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
				token_name = VALUES(token_name),
				total_requests = total_requests + VALUES(total_requests),
				successful_requests = successful_requests + VALUES(successful_requests),
				failed_requests = failed_requests + VALUES(failed_requests),
				total_tokens = total_tokens + VALUES(total_tokens),
				prompt_tokens = prompt_tokens + VALUES(prompt_tokens),
				completion_tokens = completion_tokens + VALUES(completion_tokens),
				prompt_tokens_cache = prompt_tokens_cache + VALUES(prompt_tokens_cache),
				total_quota = total_quota + VALUES(total_quota),
				updated_time = VALUES(updated_time)
		`, date, tokenId, tokenName, modelName, totalRequests, successfulRequests, failedRequests,
			totalTokens, promptTokens, completionTokens, cacheTokens, totalQuota, now, now).Error
	} else if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		return DB.Exec(`
			INSERT INTO usage_statistics
			(date, token_id, token_name, model_name, total_requests, successful_requests, failed_requests,
			 total_tokens, prompt_tokens, completion_tokens, prompt_tokens_cache, total_quota, created_time, updated_time)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (date, token_id, model_name) DO UPDATE SET
				token_name = EXCLUDED.token_name,
				total_requests = usage_statistics.total_requests + EXCLUDED.total_requests,
				successful_requests = usage_statistics.successful_requests + EXCLUDED.successful_requests,
				failed_requests = usage_statistics.failed_requests + EXCLUDED.failed_requests,
				total_tokens = usage_statistics.total_tokens + EXCLUDED.total_tokens,
				prompt_tokens = usage_statistics.prompt_tokens + EXCLUDED.prompt_tokens,
				completion_tokens = usage_statistics.completion_tokens + EXCLUDED.completion_tokens,
				prompt_tokens_cache = usage_statistics.prompt_tokens_cache + EXCLUDED.prompt_tokens_cache,
				total_quota = usage_statistics.total_quota + EXCLUDED.total_quota,
				updated_time = EXCLUDED.updated_time
		`, date, tokenId, tokenName, modelName, totalRequests, successfulRequests, failedRequests,
			totalTokens, promptTokens, completionTokens, cacheTokens, totalQuota, now, now).Error
	} else {
		// SQLite —— 先查后插/更新
		var existing UsageStatistics
		err := DB.Where("date = ? AND token_id = ? AND model_name = ?", date, tokenId, modelName).First(&existing).Error
		if err == nil {
			updates := map[string]interface{}{
				"token_name":          tokenName,
				"total_requests":      existing.TotalRequests + totalRequests,
				"successful_requests": existing.SuccessfulRequests + successfulRequests,
				"failed_requests":     existing.FailedRequests + failedRequests,
				"total_tokens":        existing.TotalTokens + totalTokens,
				"prompt_tokens":       existing.PromptTokens + promptTokens,
				"completion_tokens":   existing.CompletionTokens + completionTokens,
				"prompt_tokens_cache": existing.PromptTokensCache + cacheTokens,
				"total_quota":         existing.TotalQuota + totalQuota,
				"updated_time":        now,
			}
			return DB.Model(&existing).Updates(updates).Error
		}
		// 记录不存在，直接插入（其它错误也走插入路径，靠唯一约束兜底）
		newRecord := UsageStatistics{
			Date:               date,
			TokenId:            tokenId,
			TokenName:          tokenName,
			ModelName:          modelName,
			TotalRequests:      totalRequests,
			SuccessfulRequests: successfulRequests,
			FailedRequests:     failedRequests,
			TotalTokens:        totalTokens,
			PromptTokens:       promptTokens,
			CompletionTokens:   completionTokens,
			PromptTokensCache:  cacheTokens,
			TotalQuota:         totalQuota,
			CreatedTime:        now,
			UpdatedTime:        now,
		}
		return DB.Create(&newRecord).Error
	}
}

// RecordUsageStatistics 记录用量统计，从日志记录路径调用
func RecordUsageStatistics(tokenId int, tokenName, modelName string,
	promptTokens, completionTokens, cacheTokens int, quota int, isSuccess bool) error {

	if tokenId <= 0 || modelName == "" {
		return errors.New("invalid parameters for usage statistics")
	}

	date := time.Now().Format("2006-01-02")
	totalTokens := promptTokens + completionTokens
	totalRequests := 1
	successfulRequests := 0
	failedRequests := 0

	if isSuccess {
		successfulRequests = 1
	} else {
		failedRequests = 1
	}

	return UpsertUsageStatistics(date, tokenId, tokenName, modelName,
		totalRequests, successfulRequests, failedRequests,
		totalTokens, promptTokens, completionTokens, cacheTokens, quota)
}

// GetUsageStatisticsSummary 获取用量统计摘要（按日）
func GetUsageStatisticsSummary(startDate, endDate string, tokenId int, modelName string) (map[string]interface{}, error) {
	query := DB.Model(&UsageStatistics{})

	if startDate != "" {
		query = query.Where("date >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("date <= ?", endDate)
	}
	if tokenId > 0 {
		query = query.Where("token_id = ?", tokenId)
	}
	if modelName != "" {
		query = query.Where("model_name LIKE ?", "%"+modelName+"%")
	}
	query = applyStatsExclusionGorm(query, "")

	var result struct {
		TotalRequests      int64   `json:"total_requests"`
		SuccessfulRequests int64   `json:"successful_requests"`
		FailedRequests     int64   `json:"failed_requests"`
		TotalTokens        int64   `json:"total_tokens"`
		PromptTokens       int64   `json:"prompt_tokens"`
		CompletionTokens   int64   `json:"completion_tokens"`
		PromptTokensCache  int64   `json:"prompt_tokens_cache"`
		TotalQuota         int64   `json:"total_quota"`
	}

	err := query.Select(`
		COALESCE(SUM(total_requests), 0) as total_requests,
		COALESCE(SUM(successful_requests), 0) as successful_requests,
		COALESCE(SUM(failed_requests), 0) as failed_requests,
		COALESCE(SUM(total_tokens), 0) as total_tokens,
		COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
		COALESCE(SUM(completion_tokens), 0) as completion_tokens,
		COALESCE(SUM(prompt_tokens_cache), 0) as prompt_tokens_cache,
		COALESCE(SUM(total_quota), 0) as total_quota
	`).Scan(&result).Error
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_requests":       result.TotalRequests,
		"successful_requests":  result.SuccessfulRequests,
		"failed_requests":      result.FailedRequests,
		"total_tokens":         result.TotalTokens,
		"prompt_tokens":        result.PromptTokens,
		"completion_tokens":    result.CompletionTokens,
		"prompt_tokens_cache":  result.PromptTokensCache,
		"total_quota":          result.TotalQuota,
		"avg_prompt_tokens":    safeDiv(result.PromptTokens, result.TotalRequests),
		"avg_completion_tokens": safeDiv(result.CompletionTokens, result.TotalRequests),
	}, nil
}

// GetMonthlyUsageStatisticsSummary 获取月度用量统计摘要
func GetMonthlyUsageStatisticsSummary(startDate, endDate string, tokenId int, modelName string) (map[string]interface{}, error) {
	conditions := ""
	params := []interface{}{}
	if startDate != "" {
		conditions += " AND date >= ?"
		params = append(params, startDate+"-01")
	}
	if endDate != "" && len(endDate) >= 7 {
		year := endDate[0:4]
		month := endDate[5:7]
		conditions += " AND date <= ?"
		params = append(params, year+"-"+month+"-31")
	}
	if tokenId > 0 {
		conditions += " AND token_id = ?"
		params = append(params, tokenId)
	}
	if modelName != "" {
		conditions += " AND model_name LIKE ?"
		params = append(params, "%"+modelName+"%")
	}
	conditions, params = appendStatsExclusionCondition(conditions, "token_id", params)

	sql := `
		SELECT
			COALESCE(SUM(total_requests), 0) as total_requests,
			COALESCE(SUM(successful_requests), 0) as successful_requests,
			COALESCE(SUM(failed_requests), 0) as failed_requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) as completion_tokens,
			COALESCE(SUM(prompt_tokens_cache), 0) as prompt_tokens_cache,
			COALESCE(SUM(total_quota), 0) as total_quota
		FROM usage_statistics
		WHERE 1=1` + conditions
	var result struct {
		TotalRequests      int64 `json:"total_requests"`
		SuccessfulRequests int64 `json:"successful_requests"`
		FailedRequests     int64 `json:"failed_requests"`
		TotalTokens        int64 `json:"total_tokens"`
		PromptTokens       int64 `json:"prompt_tokens"`
		CompletionTokens   int64 `json:"completion_tokens"`
		PromptTokensCache  int64 `json:"prompt_tokens_cache"`
		TotalQuota         int64 `json:"total_quota"`
	}
	err := DB.Raw(sql, params...).Scan(&result).Error
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"total_requests":        result.TotalRequests,
		"successful_requests":   result.SuccessfulRequests,
		"failed_requests":       result.FailedRequests,
		"total_tokens":          result.TotalTokens,
		"prompt_tokens":         result.PromptTokens,
		"completion_tokens":     result.CompletionTokens,
		"prompt_tokens_cache":   result.PromptTokensCache,
		"total_quota":           result.TotalQuota,
		"avg_prompt_tokens":     safeDiv(result.PromptTokens, result.TotalRequests),
		"avg_completion_tokens": safeDiv(result.CompletionTokens, result.TotalRequests),
	}, nil
}

// GetUserUsageStatistics 获取指定用户的用量统计（按日）
func GetUserUsageStatistics(userId int, startDate, endDate string, tokenId int, modelName string, page, pageSize int) ([]*UsageStatistics, int64, error) {
	if userId <= 0 {
		return nil, 0, errors.New("invalid user id")
	}
	var statistics []*UsageStatistics
	var total int64

	query := DB.Model(&UsageStatistics{}).
		Joins("LEFT JOIN tokens ON tokens.id = usage_statistics.token_id").
		Where("tokens.user_id = ?", userId)

	if startDate != "" {
		query = query.Where("usage_statistics.date >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("usage_statistics.date <= ?", endDate)
	}
	if tokenId > 0 {
		query = query.Where("usage_statistics.token_id = ?", tokenId)
	}
	if modelName != "" {
		query = query.Where("usage_statistics.model_name LIKE ?", "%"+modelName+"%")
	}
	query = applyStatsExclusionGorm(query, "usage_statistics")

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	err = query.Order("usage_statistics.date DESC, usage_statistics.token_id ASC, usage_statistics.model_name ASC").
		Offset(offset).Limit(pageSize).Find(&statistics).Error
	return statistics, total, err
}

// GetUserMonthlyUsageStatistics 获取指定用户的月度用量统计
func GetUserMonthlyUsageStatistics(userId int, startDate, endDate string, tokenId int, modelName string, page, pageSize int) ([]*UsageStatistics, int64, error) {
	if userId <= 0 {
		return nil, 0, errors.New("invalid user id")
	}
	var statistics []*UsageStatistics
	var total int64

	conditions := " AND t.user_id = ?"
	params := []interface{}{userId}
	if startDate != "" {
		conditions += " AND u.date >= ?"
		params = append(params, startDate+"-01")
	}
	if endDate != "" && len(endDate) >= 7 {
		year := endDate[0:4]
		month := endDate[5:7]
		conditions += " AND u.date <= ?"
		params = append(params, year+"-"+month+"-31")
	}
	if tokenId > 0 {
		conditions += " AND u.token_id = ?"
		params = append(params, tokenId)
	}
	if modelName != "" {
		conditions += " AND u.model_name LIKE ?"
		params = append(params, "%"+modelName+"%")
	}
	conditions, params = appendStatsExclusionCondition(conditions, "u.token_id", params)

	sql := `
		SELECT
			MAX(u.id) as id,
			SUBSTR(u.date, 1, 7) as date,
			u.token_id,
			u.token_name,
			u.model_name,
			SUM(u.total_requests) as total_requests,
			SUM(u.successful_requests) as successful_requests,
			SUM(u.failed_requests) as failed_requests,
			SUM(u.total_tokens) as total_tokens,
			SUM(u.prompt_tokens) as prompt_tokens,
			SUM(u.completion_tokens) as completion_tokens,
			SUM(u.prompt_tokens_cache) as prompt_tokens_cache,
			SUM(u.total_quota) as total_quota,
			MAX(u.created_time) as created_time,
			MAX(u.updated_time) as updated_time
		FROM usage_statistics u
		LEFT JOIN tokens t ON t.id = u.token_id
		WHERE 1=1` + conditions + `
		GROUP BY SUBSTR(u.date, 1, 7), u.token_id, u.token_name, u.model_name
		ORDER BY date DESC, u.token_id ASC, u.model_name ASC
	`
	countSQL := `
		SELECT COUNT(*) as count FROM (
			SELECT 1
			FROM usage_statistics u
			LEFT JOIN tokens t ON t.id = u.token_id
			WHERE 1=1` + conditions + `
			GROUP BY SUBSTR(u.date, 1, 7), u.token_id, u.token_name, u.model_name
		) as grouped_data
	`
	var countResult struct {
		Count int64 `json:"count"`
	}
	err := DB.Raw(countSQL, params...).Scan(&countResult).Error
	if err != nil {
		return nil, 0, err
	}
	total = countResult.Count

	offset := (page - 1) * pageSize
	limitSQL := sql + " LIMIT ? OFFSET ?"
	params = append(params, pageSize, offset)
	err = DB.Raw(limitSQL, params...).Scan(&statistics).Error
	return statistics, total, err
}

// GetUserUsageStatisticsSummary 获取指定用户用量统计摘要
func GetUserUsageStatisticsSummary(userId int, startDate, endDate string, tokenId int, modelName string) (map[string]interface{}, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	query := DB.Model(&UsageStatistics{}).
		Joins("LEFT JOIN tokens ON tokens.id = usage_statistics.token_id").
		Where("tokens.user_id = ?", userId)
	if startDate != "" {
		query = query.Where("usage_statistics.date >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("usage_statistics.date <= ?", endDate)
	}
	if tokenId > 0 {
		query = query.Where("usage_statistics.token_id = ?", tokenId)
	}
	if modelName != "" {
		query = query.Where("usage_statistics.model_name LIKE ?", "%"+modelName+"%")
	}
	query = applyStatsExclusionGorm(query, "usage_statistics")
	var result struct {
		TotalRequests      int64 `json:"total_requests"`
		SuccessfulRequests int64 `json:"successful_requests"`
		FailedRequests     int64 `json:"failed_requests"`
		TotalTokens        int64 `json:"total_tokens"`
		PromptTokens       int64 `json:"prompt_tokens"`
		CompletionTokens   int64 `json:"completion_tokens"`
		PromptTokensCache  int64 `json:"prompt_tokens_cache"`
		TotalQuota         int64 `json:"total_quota"`
	}
	err := query.Select(`
		COALESCE(SUM(usage_statistics.total_requests), 0) as total_requests,
		COALESCE(SUM(usage_statistics.successful_requests), 0) as successful_requests,
		COALESCE(SUM(usage_statistics.failed_requests), 0) as failed_requests,
		COALESCE(SUM(usage_statistics.total_tokens), 0) as total_tokens,
		COALESCE(SUM(usage_statistics.prompt_tokens), 0) as prompt_tokens,
		COALESCE(SUM(usage_statistics.completion_tokens), 0) as completion_tokens,
		COALESCE(SUM(usage_statistics.prompt_tokens_cache), 0) as prompt_tokens_cache,
		COALESCE(SUM(usage_statistics.total_quota), 0) as total_quota
	`).Scan(&result).Error
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"total_requests":        result.TotalRequests,
		"successful_requests":   result.SuccessfulRequests,
		"failed_requests":       result.FailedRequests,
		"total_tokens":          result.TotalTokens,
		"prompt_tokens":         result.PromptTokens,
		"completion_tokens":     result.CompletionTokens,
		"prompt_tokens_cache":   result.PromptTokensCache,
		"total_quota":           result.TotalQuota,
		"avg_prompt_tokens":     safeDiv(result.PromptTokens, result.TotalRequests),
		"avg_completion_tokens": safeDiv(result.CompletionTokens, result.TotalRequests),
	}, nil
}

// GetUserMonthlyUsageStatisticsSummary 获取指定用户月度用量统计摘要
func GetUserMonthlyUsageStatisticsSummary(userId int, startDate, endDate string, tokenId int, modelName string) (map[string]interface{}, error) {
	if userId <= 0 {
		return nil, errors.New("invalid user id")
	}
	conditions := " AND t.user_id = ?"
	params := []interface{}{userId}
	if startDate != "" {
		conditions += " AND u.date >= ?"
		params = append(params, startDate+"-01")
	}
	if endDate != "" && len(endDate) >= 7 {
		year := endDate[0:4]
		month := endDate[5:7]
		conditions += " AND u.date <= ?"
		params = append(params, year+"-"+month+"-31")
	}
	if tokenId > 0 {
		conditions += " AND u.token_id = ?"
		params = append(params, tokenId)
	}
	if modelName != "" {
		conditions += " AND u.model_name LIKE ?"
		params = append(params, "%"+modelName+"%")
	}
	conditions, params = appendStatsExclusionCondition(conditions, "u.token_id", params)
	sql := `
		SELECT
			COALESCE(SUM(u.total_requests), 0) as total_requests,
			COALESCE(SUM(u.successful_requests), 0) as successful_requests,
			COALESCE(SUM(u.failed_requests), 0) as failed_requests,
			COALESCE(SUM(u.total_tokens), 0) as total_tokens,
			COALESCE(SUM(u.prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(u.completion_tokens), 0) as completion_tokens,
			COALESCE(SUM(u.prompt_tokens_cache), 0) as prompt_tokens_cache,
			COALESCE(SUM(u.total_quota), 0) as total_quota
		FROM usage_statistics u
		LEFT JOIN tokens t ON t.id = u.token_id
		WHERE 1=1` + conditions
	var result struct {
		TotalRequests      int64 `json:"total_requests"`
		SuccessfulRequests int64 `json:"successful_requests"`
		FailedRequests     int64 `json:"failed_requests"`
		TotalTokens        int64 `json:"total_tokens"`
		PromptTokens       int64 `json:"prompt_tokens"`
		CompletionTokens   int64 `json:"completion_tokens"`
		PromptTokensCache  int64 `json:"prompt_tokens_cache"`
		TotalQuota         int64 `json:"total_quota"`
	}
	err := DB.Raw(sql, params...).Scan(&result).Error
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"total_requests":        result.TotalRequests,
		"successful_requests":   result.SuccessfulRequests,
		"failed_requests":       result.FailedRequests,
		"total_tokens":          result.TotalTokens,
		"prompt_tokens":         result.PromptTokens,
		"completion_tokens":     result.CompletionTokens,
		"prompt_tokens_cache":   result.PromptTokensCache,
		"total_quota":           result.TotalQuota,
		"avg_prompt_tokens":     safeDiv(result.PromptTokens, result.TotalRequests),
		"avg_completion_tokens": safeDiv(result.CompletionTokens, result.TotalRequests),
	}, nil
}

// safeDiv 安全除法，分母为 0 时返回 0
func safeDiv(numerator, denominator int64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

// splitTokenIds 把逗号分隔的 token id 字符串切成切片（供分布查询复用）
func splitTokenIds(tokenIds string) []string {
	if tokenIds == "" {
		return nil
	}
	parts := strings.Split(tokenIds, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// statsExcludedTokenIds 返回全局配置的统计排除 token id 列表（已去重、仅保留正整数）。
// 空列表表示不排除任何令牌。所有统计查询（日/月/排序/分布）都会排除这些 token。
func statsExcludedTokenIds() []int {
	raw := common.StatsExcludedTokenIds
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	seen := make(map[int]bool, len(parts))
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.Atoi(p)
		if err != nil || id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

// applyStatsExclusionGorm 给 GORM 查询附加全局统计排除条件：
//  - token_id NOT IN (配置的排除令牌)
func applyStatsExclusionGorm(query *gorm.DB, tablePrefix string) *gorm.DB {
	tokenCol := "token_id"
	if tablePrefix != "" {
		tokenCol = tablePrefix + ".token_id"
	}
	ids := statsExcludedTokenIds()
	if len(ids) > 0 {
		query = query.Where(tokenCol+" NOT IN ?", ids)
	}
	return query
}

// appendStatsExclusionCondition 给原生 SQL 的 conditions 片段追加全局统计排除条件：
//  - tokenCol NOT IN (配置的排除令牌)
// tokenCol 为完整列名（可含表前缀，如 "u.token_id"），params 为该查询的参数切片。
func appendStatsExclusionCondition(conditions, tokenCol string, params []interface{}) (string, []interface{}) {
	ids := statsExcludedTokenIds()
	if len(ids) > 0 {
		conditions += " AND " + tokenCol + " NOT IN (?)"
		params = append(params, ids)
	}
	return conditions, params
}
