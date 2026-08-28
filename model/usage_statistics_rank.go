package model

// usage_statistics_rank.go —— 用量排序统计查询（MySQL 专用）。
//
// 这些查询依赖 MySQL 专用函数（FROM_UNIXTIME/DATE/CAST AS UNSIGNED/SUBSTRING_INDEX/CONCAT/
// NULLIF/ROUND）并跨库 JOIN logs 表。按用户决策（见 memory: feedback-rank-mysql-only），
// 这是 AGENTS.md 三库兼容规则的明确例外：仅在 MySQL + 主库/日志库同实例部署下可用。
//
// 移植自 MIXAPI model/usage_statistics.go 的 GetRankUsageStatistics 系列函数，原样保留。

// rankDateCondition 构建日期范围条件片段（usage_statistics.date 列）与参数。
// startDate/endDate 支持 "YYYY-MM-DD"（10 位）或 "YYYY-MM"（>=7 位，按月）。
func rankDateCondition(startDate, endDate string) (string, []interface{}) {
	conditions := ""
	var params []interface{}
	if startDate != "" {
		if len(startDate) == 10 {
			conditions += " AND date >= ?"
			params = append(params, startDate)
		} else if len(startDate) >= 7 {
			conditions += " AND date >= ?"
			params = append(params, startDate[0:7]+"-01")
		}
	}
	if endDate != "" {
		if len(endDate) == 10 {
			conditions += " AND date <= ?"
			params = append(params, endDate)
		} else if len(endDate) >= 7 {
			year := endDate[0:4]
			month := endDate[5:7]
			conditions += " AND date <= ?"
			params = append(params, year+"-"+month+"-31")
		}
	}
	return conditions, params
}

// rankLogsCondition 构建日期范围条件片段（logs.created_at 列，MySQL FROM_UNIXTIME）与参数。
func rankLogsCondition(startDate, endDate string) (string, []interface{}) {
	conditions := ""
	var params []interface{}
	if startDate != "" {
		if len(startDate) == 10 {
			conditions += " AND DATE(FROM_UNIXTIME(created_at)) >= '" + startDate + "'"
		} else if len(startDate) >= 7 {
			conditions += " AND DATE(FROM_UNIXTIME(created_at)) >= '" + startDate[0:7] + "-01'"
		}
	}
	if endDate != "" {
		if len(endDate) == 10 {
			conditions += " AND DATE(FROM_UNIXTIME(created_at)) <= '" + endDate + "'"
		} else if len(endDate) >= 7 {
			year := endDate[0:4]
			month := endDate[5:7]
			conditions += " AND DATE(FROM_UNIXTIME(created_at)) <= '" + year + "-" + month + "-31'"
		}
	}
	return conditions, params
}

// GetRankUsageStatistics 获取用量排序统计数据（管理员维度，按 token 聚合）
func GetRankUsageStatistics(startDate, endDate string, tokenIds string, modelName string, groupBy string, page, pageSize int) ([]*UsageStatistics, int64, error) {
	var statistics []*UsageStatistics
	var total int64

	db := DB.Model(&UsageStatistics{})

	conditions, params := rankDateCondition(startDate, endDate)
	logsConditions, _ := rankLogsCondition(startDate, endDate)

	if tokenIds != "" {
		conditions += " AND token_id IN (?)"
		params = append(params, tokenIds)
	}
	if modelName != "" {
		conditions += " AND model_name LIKE ?"
		params = append(params, "%"+modelName+"%")
	}
	conditions, params = appendStatsExclusionCondition(conditions, "token_id", params)

	var sql string
	var countSQL string

	if groupBy == "exact" {
		sql = `
			SELECT
				MAX(us.id) as id,
				us.token_id,
				us.token_name,
				MAX(us.model_name) as model_name,
				SUM(us.total_requests) as total_requests,
				SUM(us.successful_requests) as successful_requests,
				SUM(us.failed_requests) as failed_requests,
				SUM(us.total_tokens) as total_tokens,
				SUM(us.prompt_tokens) as prompt_tokens,
				SUM(us.completion_tokens) as completion_tokens,
				SUM(us.prompt_tokens_cache) as prompt_tokens_cache,
				SUM(us.total_quota) as total_quota,
				CAST(ROUND(SUM(us.prompt_tokens) * 1.0 / NULLIF(SUM(us.total_requests), 0)) AS UNSIGNED) as avg_prompt_tokens,
				MAX(CASE WHEN l.min_prompt_tokens > 0 THEN l.min_prompt_tokens END) as min_prompt_tokens,
				MAX(l.max_prompt_tokens) as max_prompt_tokens,
				CAST(ROUND(SUM(us.completion_tokens) * 1.0 / NULLIF(SUM(us.total_requests), 0)) AS UNSIGNED) as avg_completion_tokens,
				MAX(CASE WHEN l.min_completion_tokens > 0 THEN l.min_completion_tokens END) as min_completion_tokens,
				MAX(l.max_completion_tokens) as max_completion_tokens,
				MAX(l.question_count) as question_count,
				MAX(CASE WHEN l.min_requests_per_question > 0 THEN l.min_requests_per_question END) as min_requests_per_question,
				MAX(l.max_requests_per_question) as max_requests_per_question,
				MAX(us.created_time) as created_time,
				MAX(us.updated_time) as updated_time
			FROM usage_statistics us
			LEFT JOIN (
				SELECT
					token_name,
					COUNT(DISTINCT user_input) as question_count,
					MIN(CASE WHEN cnt > 0 THEN cnt END) as min_requests_per_question,
					MAX(cnt) as max_requests_per_question,
					MIN(CASE WHEN prompt_tokens > 0 THEN prompt_tokens END) as min_prompt_tokens,
					MAX(prompt_tokens) as max_prompt_tokens,
					MIN(CASE WHEN completion_tokens > 0 THEN completion_tokens END) as min_completion_tokens,
					MAX(completion_tokens) as max_completion_tokens
				FROM (
					SELECT token_name, user_input, COUNT(*) as cnt,
					       CAST(ROUND(AVG(prompt_tokens)) AS UNSIGNED) as prompt_tokens,
					       CAST(ROUND(AVG(completion_tokens)) AS UNSIGNED) as completion_tokens
					FROM logs
					WHERE type = 2 AND user_input IS NOT NULL AND user_input != '' AND user_input != '[]' AND user_input != 'null' AND LENGTH(user_input) > 2` + logsConditions + `
					GROUP BY token_name, user_input
				) as question_counts
				GROUP BY token_name
			) l ON l.token_name = us.token_name
			WHERE 1=1` + conditions + `
			GROUP BY us.token_id, us.token_name
			ORDER BY SUM(us.total_tokens) DESC
		`

		countSQL = `
			SELECT COUNT(*) as count FROM (
				SELECT 1
				FROM usage_statistics
				WHERE 1=1` + conditions + `
				GROUP BY token_id, token_name
			) as grouped_data
		`
	} else {
		sql = `
			SELECT
				MAX(us.id) as id,
				0 as token_id,
				MAX(CONCAT(SUBSTRING_INDEX(us.token_name, '-', 1), '*')) as token_name,
				MAX(us.model_name) as model_name,
				SUM(us.total_requests) as total_requests,
				SUM(us.successful_requests) as successful_requests,
				SUM(us.failed_requests) as failed_requests,
				SUM(us.total_tokens) as total_tokens,
				SUM(us.prompt_tokens) as prompt_tokens,
				SUM(us.completion_tokens) as completion_tokens,
				SUM(us.prompt_tokens_cache) as prompt_tokens_cache,
				SUM(us.total_quota) as total_quota,
				CAST(ROUND(SUM(us.prompt_tokens) * 1.0 / NULLIF(SUM(us.total_requests), 0)) AS UNSIGNED) as avg_prompt_tokens,
				MAX(l.min_prompt_tokens) as min_prompt_tokens,
				MAX(l.max_prompt_tokens) as max_prompt_tokens,
				CAST(ROUND(SUM(us.completion_tokens) * 1.0 / NULLIF(SUM(us.total_requests), 0)) AS UNSIGNED) as avg_completion_tokens,
				MAX(l.min_completion_tokens) as min_completion_tokens,
				MAX(l.max_completion_tokens) as max_completion_tokens,
				MAX(l.question_count) as question_count,
				MAX(CASE WHEN l.min_requests_per_question > 0 THEN l.min_requests_per_question END) as min_requests_per_question,
				MAX(l.max_requests_per_question) as max_requests_per_question,
				MAX(us.created_time) as created_time,
				MAX(us.updated_time) as updated_time
			FROM usage_statistics us
			LEFT JOIN (
				SELECT
					SUBSTRING_INDEX(token_name, '-', 1) as token_prefix,
					COUNT(DISTINCT user_input) as question_count,
					MIN(CASE WHEN cnt > 0 THEN cnt END) as min_requests_per_question,
					MAX(cnt) as max_requests_per_question,
					MIN(CASE WHEN prompt_tokens > 0 THEN prompt_tokens END) as min_prompt_tokens,
					MAX(prompt_tokens) as max_prompt_tokens,
					MIN(CASE WHEN completion_tokens > 0 THEN completion_tokens END) as min_completion_tokens,
					MAX(completion_tokens) as max_completion_tokens
				FROM (
					SELECT token_name, user_input, COUNT(*) as cnt,
					       CAST(ROUND(AVG(prompt_tokens)) AS UNSIGNED) as prompt_tokens,
					       CAST(ROUND(AVG(completion_tokens)) AS UNSIGNED) as completion_tokens
					FROM logs
					WHERE type = 2 AND user_input IS NOT NULL AND user_input != '' AND user_input != '[]' AND user_input != 'null' AND LENGTH(user_input) > 2` + logsConditions + `
					GROUP BY token_name, user_input
				) as question_counts
				GROUP BY SUBSTRING_INDEX(token_name, '-', 1)
			) l ON l.token_prefix = SUBSTRING_INDEX(us.token_name, '-', 1)
			WHERE 1=1` + conditions + `
			GROUP BY SUBSTRING_INDEX(us.token_name, '-', 1)
			ORDER BY SUM(us.total_tokens) DESC
		`

		countSQL = `
			SELECT COUNT(DISTINCT SUBSTRING_INDEX(token_name, '-', 1)) as count
			FROM usage_statistics
			WHERE 1=1` + conditions
	}

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

// GetRankUsageStatisticsSummary 获取用量排序统计摘要（管理员维度）
func GetRankUsageStatisticsSummary(startDate, endDate string, tokenIds string, modelName string, groupBy string) (map[string]interface{}, error) {
	db := DB.Model(&UsageStatistics{})

	conditions, params := rankDateCondition(startDate, endDate)
	logsConditions, logsParams := rankLogsConditionTimestamp(startDate, endDate)

	if tokenIds != "" {
		conditions += " AND token_id IN (?)"
		params = append(params, tokenIds)
	}
	if modelName != "" {
		conditions += " AND model_name LIKE ?"
		params = append(params, "%"+modelName+"%")
	}
	conditions, params = appendStatsExclusionCondition(conditions, "token_id", params)

	groupField := "token_id, token_name"
	if groupBy == "prefix" {
		groupField = "SUBSTRING_INDEX(token_name, '-', 1)"
	}

	joinCondition := "us.token_name"
	if groupBy == "prefix" {
		joinCondition = "SUBSTRING_INDEX(us.token_name, '-', 1)"
	}

	usSelectFields := "token_id, token_name"
	usGroupFields := groupField
	if groupBy == "prefix" {
		usSelectFields = "0 as token_id, MAX(token_name) as token_name"
		usGroupFields = "SUBSTRING_INDEX(token_name, '-', 1)"
	}

	logsGroupField := groupField
	logsSelectFields := groupField
	if groupBy == "prefix" {
		logsGroupField = "SUBSTRING_INDEX(token_name, '-', 1)"
		logsSelectFields = "SUBSTRING_INDEX(token_name, '-', 1)"
	} else {
		logsGroupField = "token_name"
		logsSelectFields = "token_name"
	}

	summarySQL := `
		SELECT
			SUM(us.total_requests) as total_requests,
			SUM(us.successful_requests) as successful_requests,
			SUM(us.failed_requests) as failed_requests,
			SUM(us.total_tokens) as total_tokens,
			SUM(us.prompt_tokens) as prompt_tokens,
			SUM(us.completion_tokens) as completion_tokens,
			SUM(us.prompt_tokens_cache) as prompt_tokens_cache,
			SUM(us.total_quota) as total_quota,
			COALESCE(SUM(l.question_count), 0) as question_count,
			COALESCE(MIN(l.min_prompt_tokens), 0) as min_prompt_tokens,
			COALESCE(MAX(l.max_prompt_tokens), 0) as max_prompt_tokens,
			COALESCE(MIN(l.min_completion_tokens), 0) as min_completion_tokens,
			COALESCE(MAX(l.max_completion_tokens), 0) as max_completion_tokens,
			COALESCE(MIN(l.min_requests_per_question), 0) as min_requests_per_question,
			COALESCE(MAX(l.max_requests_per_question), 0) as max_requests_per_question
		FROM (
			SELECT
				` + usSelectFields + `,
				SUM(total_requests) as total_requests,
				SUM(successful_requests) as successful_requests,
				SUM(failed_requests) as failed_requests,
				SUM(total_tokens) as total_tokens,
				SUM(prompt_tokens) as prompt_tokens,
				SUM(completion_tokens) as completion_tokens,
				SUM(prompt_tokens_cache) as prompt_tokens_cache,
				SUM(total_quota) as total_quota
			FROM usage_statistics
			WHERE 1=1` + conditions + `
			GROUP BY ` + usGroupFields + `
		) us
		LEFT JOIN (
			SELECT
				` + logsSelectFields + ` as group_key,
				COUNT(DISTINCT user_input) as question_count,
				MIN(CASE WHEN cnt > 0 THEN cnt END) as min_requests_per_question,
				MAX(cnt) as max_requests_per_question,
				MIN(CASE WHEN prompt_tokens > 0 THEN prompt_tokens END) as min_prompt_tokens,
				MAX(prompt_tokens) as max_prompt_tokens,
				MIN(CASE WHEN completion_tokens > 0 THEN completion_tokens END) as min_completion_tokens,
				MAX(completion_tokens) as max_completion_tokens
				FROM (
					SELECT token_name, user_input, COUNT(*) as cnt,
					       CAST(ROUND(AVG(prompt_tokens)) AS UNSIGNED) as prompt_tokens,
					       CAST(ROUND(AVG(completion_tokens)) AS UNSIGNED) as completion_tokens
					FROM logs
					WHERE type = 2 AND user_input IS NOT NULL AND user_input != '' AND user_input != '[]' AND user_input != 'null' AND LENGTH(user_input) > 2` + logsConditions + `
					GROUP BY token_name, user_input
				) as question_counts
			GROUP BY ` + logsGroupField + `
		) l ON l.group_key = ` + joinCondition + `
	`

	allParams := append([]interface{}{}, params...)
	allParams = append(allParams, logsParams...)

	var result struct {
		TotalRequests          int `json:"total_requests"`
		SuccessfulRequests     int `json:"successful_requests"`
		FailedRequests         int `json:"failed_requests"`
		TotalTokens            int `json:"total_tokens"`
		PromptTokens           int `json:"prompt_tokens"`
		CompletionTokens       int `json:"completion_tokens"`
		PromptTokensCache      int `json:"prompt_tokens_cache"`
		TotalQuota             int `json:"total_quota"`
		QuestionCount          int `json:"question_count"`
		MinPromptTokens        int `json:"min_prompt_tokens"`
		MaxPromptTokens        int `json:"max_prompt_tokens"`
		MinCompletionTokens    int `json:"min_completion_tokens"`
		MaxCompletionTokens    int `json:"max_completion_tokens"`
		MinRequestsPerQuestion int `json:"min_requests_per_question"`
		MaxRequestsPerQuestion int `json:"max_requests_per_question"`
	}

	err := db.Raw(summarySQL, allParams...).Scan(&result).Error
	if err != nil {
		return nil, err
	}

	summary := map[string]interface{}{
		"total_requests":            result.TotalRequests,
		"successful_requests":       result.SuccessfulRequests,
		"failed_requests":           result.FailedRequests,
		"success_rate":              0.0,
		"total_tokens":              result.TotalTokens,
		"total_prompt_tokens":       result.PromptTokens,
		"total_completion_tokens":   result.CompletionTokens,
		"total_cache_tokens":        result.PromptTokensCache,
		"total_quota":               result.TotalQuota,
		"question_count":            result.QuestionCount,
		"min_prompt_tokens":         result.MinPromptTokens,
		"max_prompt_tokens":         result.MaxPromptTokens,
		"min_completion_tokens":     result.MinCompletionTokens,
		"max_completion_tokens":     result.MaxCompletionTokens,
		"min_requests_per_question": result.MinRequestsPerQuestion,
		"max_requests_per_question": result.MaxRequestsPerQuestion,
	}

	if result.TotalRequests > 0 {
		summary["success_rate"] = float64(result.SuccessfulRequests) / float64(result.TotalRequests) * 100
		summary["avg_prompt_tokens"] = float64(result.PromptTokens) / float64(result.TotalRequests)
		summary["avg_completion_tokens"] = float64(result.CompletionTokens) / float64(result.TotalRequests)
	} else {
		summary["avg_prompt_tokens"] = 0.0
		summary["avg_completion_tokens"] = 0.0
	}

	if result.QuestionCount > 0 {
		summary["avg_requests_per_question"] = float64(result.TotalRequests) / float64(result.QuestionCount)
	}

	return summary, nil
}

// rankLogsConditionTimestamp 构建日期范围条件片段（logs.created_at 列，时间戳参数版本，供 summary 使用）
func rankLogsConditionTimestamp(startDate, endDate string) (string, []interface{}) {
	conditions := ""
	var params []interface{}
	if startDate != "" {
		if len(startDate) == 10 {
			conditions += " AND created_at >= ?"
			params = append(params, parseTimestamp(startDate+" 00:00:00"))
		} else if len(startDate) >= 7 {
			conditions += " AND created_at >= ?"
			params = append(params, parseTimestamp(startDate[0:7]+"-01 00:00:00"))
		}
	}
	if endDate != "" {
		if len(endDate) == 10 {
			conditions += " AND created_at <= ?"
			params = append(params, parseTimestamp(endDate+" 23:59:59"))
		} else if len(endDate) >= 7 {
			year := endDate[0:4]
			month := endDate[5:7]
			conditions += " AND created_at <= ?"
			params = append(params, parseTimestamp(year+"-"+month+"-31 23:59:59"))
		}
	}
	return conditions, params
}

// GetUserRankUsageStatistics 获取特定用户的用量排序统计数据
func GetUserRankUsageStatistics(userId int, startDate, endDate string, tokenIds string, modelName string, groupBy string, page, pageSize int) ([]*UsageStatistics, int64, error) {
	var statistics []*UsageStatistics
	var total int64

	query := DB.Table("usage_statistics").
		Joins("JOIN tokens ON usage_statistics.token_id = tokens.id").
		Where("tokens.user_id = ?", userId)

	if startDate != "" {
		if len(startDate) == 10 {
			query = query.Where("usage_statistics.date >= ?", startDate)
		} else if len(startDate) >= 7 {
			query = query.Where("usage_statistics.date >= ?", startDate[0:7]+"-01")
		}
	}
	if endDate != "" {
		if len(endDate) == 10 {
			query = query.Where("usage_statistics.date <= ?", endDate)
		} else if len(endDate) >= 7 {
			year := endDate[0:4]
			month := endDate[5:7]
			query = query.Where("usage_statistics.date <= ?", year+"-"+month+"-31")
		}
	}
	if tokenIds != "" {
		query = query.Where("usage_statistics.token_id IN (?)", tokenIds)
	}
	if modelName != "" {
		query = query.Where("usage_statistics.model_name LIKE ?", "%"+modelName+"%")
	}
	query = applyStatsExclusionGorm(query, "usage_statistics")

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	conditions := " AND tokens.user_id = ?"
	params := []interface{}{userId}
	logsConditions, _ := rankLogsCondition(startDate, endDate)

	if startDate != "" {
		if len(startDate) == 10 {
			conditions += " AND usage_statistics.date >= ?"
			params = append(params, startDate)
		} else if len(startDate) >= 7 {
			conditions += " AND usage_statistics.date >= ?"
			params = append(params, startDate[0:7]+"-01")
		}
	}
	if endDate != "" {
		if len(endDate) == 10 {
			conditions += " AND usage_statistics.date <= ?"
			params = append(params, endDate)
		} else if len(endDate) >= 7 {
			year := endDate[0:4]
			month := endDate[5:7]
			conditions += " AND usage_statistics.date <= ?"
			params = append(params, year+"-"+month+"-31")
		}
	}
	if tokenIds != "" {
		conditions += " AND usage_statistics.token_id IN (?)"
		params = append(params, tokenIds)
	}
	if modelName != "" {
		conditions += " AND usage_statistics.model_name LIKE ?"
		params = append(params, "%"+modelName+"%")
	}
	conditions, params = appendStatsExclusionCondition(conditions, "usage_statistics.token_id", params)

	var sql string
	var countSQL string

	if groupBy == "exact" {
		sql = `
			SELECT
				MAX(usage_statistics.id) as id,
				usage_statistics.token_id,
				MAX(usage_statistics.token_name) as token_name,
				MAX(usage_statistics.model_name) as model_name,
				SUM(usage_statistics.total_requests) as total_requests,
				SUM(usage_statistics.successful_requests) as successful_requests,
				SUM(usage_statistics.failed_requests) as failed_requests,
				SUM(usage_statistics.total_tokens) as total_tokens,
				SUM(usage_statistics.prompt_tokens) as prompt_tokens,
				SUM(usage_statistics.completion_tokens) as completion_tokens,
				SUM(usage_statistics.prompt_tokens_cache) as prompt_tokens_cache,
				SUM(usage_statistics.total_quota) as total_quota,
				MAX(l.avg_prompt_tokens) as avg_prompt_tokens,
				MAX(l.min_prompt_tokens) as min_prompt_tokens,
				MAX(l.max_prompt_tokens) as max_prompt_tokens,
				MAX(l.avg_completion_tokens) as avg_completion_tokens,
				MAX(l.min_completion_tokens) as min_completion_tokens,
				MAX(l.max_completion_tokens) as max_completion_tokens,
				MAX(l.question_count) as question_count,
				MAX(l.min_requests_per_question) as min_requests_per_question,
				MAX(l.max_requests_per_question) as max_requests_per_question,
				MAX(usage_statistics.created_time) as created_time,
				MAX(usage_statistics.updated_time) as updated_time
			FROM usage_statistics
			JOIN tokens ON usage_statistics.token_id = tokens.id
			LEFT JOIN (
				SELECT
					token_name,
					COUNT(DISTINCT user_input) as question_count,
					MIN(cnt) as min_requests_per_question,
					MAX(cnt) as max_requests_per_question,
					CAST(ROUND(AVG(prompt_tokens)) AS UNSIGNED) as avg_prompt_tokens,
					MIN(prompt_tokens) as min_prompt_tokens,
					MAX(prompt_tokens) as max_prompt_tokens,
					CAST(ROUND(AVG(completion_tokens)) AS UNSIGNED) as avg_completion_tokens,
					MIN(completion_tokens) as min_completion_tokens,
					MAX(completion_tokens) as max_completion_tokens
				FROM (
					SELECT token_name, user_input, COUNT(*) as cnt,
					       CAST(ROUND(AVG(prompt_tokens)) AS UNSIGNED) as prompt_tokens,
					       CAST(ROUND(AVG(completion_tokens)) AS UNSIGNED) as completion_tokens
					FROM logs
					WHERE type = 2 AND user_input IS NOT NULL AND user_input != '' AND user_input != '[]' AND user_input != 'null' AND LENGTH(user_input) > 2` + logsConditions + `
					GROUP BY token_name, user_input
				) as question_counts
				GROUP BY token_name
			) l ON l.token_name = usage_statistics.token_name
			WHERE 1=1` + conditions + `
			GROUP BY usage_statistics.token_id, usage_statistics.token_name
			ORDER BY SUM(usage_statistics.total_tokens) DESC
		`

		countSQL = `
			SELECT COUNT(*) as count FROM (
				SELECT 1
				FROM usage_statistics
				JOIN tokens ON usage_statistics.token_id = tokens.id
				WHERE 1=1` + conditions + `
				GROUP BY usage_statistics.token_id, usage_statistics.token_name
			) as grouped_data
		`
	} else {
		sql = `
			SELECT
				MAX(usage_statistics.id) as id,
				0 as token_id,
				MAX(CONCAT(SUBSTRING_INDEX(usage_statistics.token_name, '-', 1), '*')) as token_name,
				MAX(usage_statistics.model_name) as model_name,
				SUM(usage_statistics.total_requests) as total_requests,
				SUM(usage_statistics.successful_requests) as successful_requests,
				SUM(usage_statistics.failed_requests) as failed_requests,
				SUM(usage_statistics.total_tokens) as total_tokens,
				SUM(usage_statistics.prompt_tokens) as prompt_tokens,
				SUM(usage_statistics.completion_tokens) as completion_tokens,
				SUM(usage_statistics.prompt_tokens_cache) as prompt_tokens_cache,
				SUM(usage_statistics.total_quota) as total_quota,
				MAX(l.avg_prompt_tokens) as avg_prompt_tokens,
				MAX(l.min_prompt_tokens) as min_prompt_tokens,
				MAX(l.max_prompt_tokens) as max_prompt_tokens,
				MAX(l.avg_completion_tokens) as avg_completion_tokens,
				MAX(l.min_completion_tokens) as min_completion_tokens,
				MAX(l.max_completion_tokens) as max_completion_tokens,
				MAX(l.question_count) as question_count,
				MAX(l.min_requests_per_question) as min_requests_per_question,
				MAX(l.max_requests_per_question) as max_requests_per_question,
				MAX(usage_statistics.created_time) as created_time,
				MAX(usage_statistics.updated_time) as updated_time
			FROM usage_statistics
			JOIN tokens ON usage_statistics.token_id = tokens.id
			LEFT JOIN (
				SELECT
					SUBSTRING_INDEX(token_name, '-', 1) as token_prefix,
					COUNT(DISTINCT user_input) as question_count,
					MIN(cnt) as min_requests_per_question,
					MAX(cnt) as max_requests_per_question,
					CAST(ROUND(AVG(prompt_tokens)) AS UNSIGNED) as avg_prompt_tokens,
					MIN(prompt_tokens) as min_prompt_tokens,
					MAX(prompt_tokens) as max_prompt_tokens,
					CAST(ROUND(AVG(completion_tokens)) AS UNSIGNED) as avg_completion_tokens,
					MIN(completion_tokens) as min_completion_tokens,
					MAX(completion_tokens) as max_completion_tokens
				FROM (
					SELECT token_name, user_input, COUNT(*) as cnt,
					       CAST(ROUND(AVG(prompt_tokens)) AS UNSIGNED) as prompt_tokens,
					       CAST(ROUND(AVG(completion_tokens)) AS UNSIGNED) as completion_tokens
					FROM logs
					WHERE type = 2 AND user_input IS NOT NULL AND user_input != '' AND user_input != '[]' AND user_input != 'null' AND LENGTH(user_input) > 2` + logsConditions + `
					GROUP BY token_name, user_input
				) as question_counts
				GROUP BY SUBSTRING_INDEX(token_name, '-', 1)
			) l ON l.token_prefix = SUBSTRING_INDEX(usage_statistics.token_name, '-', 1)
			WHERE 1=1` + conditions + `
			GROUP BY SUBSTRING_INDEX(usage_statistics.token_name, '-', 1)
			ORDER BY SUM(usage_statistics.total_tokens) DESC
		`

		countSQL = `
			SELECT COUNT(DISTINCT SUBSTRING_INDEX(usage_statistics.token_name, '-', 1)) as count
			FROM usage_statistics
			JOIN tokens ON usage_statistics.token_id = tokens.id
			WHERE 1=1` + conditions
	}

	var countResult struct {
		Count int64 `json:"count"`
	}
	err = DB.Raw(countSQL, params...).Scan(&countResult).Error
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

// GetUserRankUsageStatisticsSummary 获取特定用户的用量排序统计摘要
func GetUserRankUsageStatisticsSummary(userId int, startDate, endDate string, tokenIds string, modelName string, groupBy string) (map[string]interface{}, error) {
	db := DB.Table("usage_statistics").
		Joins("JOIN tokens ON usage_statistics.token_id = tokens.id").
		Where("tokens.user_id = ?", userId)

	conditions := " AND tokens.user_id = ?"
	params := []interface{}{userId}

	if startDate != "" {
		if len(startDate) == 10 {
			conditions += " AND usage_statistics.date >= ?"
			params = append(params, startDate)
		} else if len(startDate) >= 7 {
			conditions += " AND usage_statistics.date >= ?"
			params = append(params, startDate[0:7]+"-01")
		}
	}
	if endDate != "" {
		if len(endDate) == 10 {
			conditions += " AND usage_statistics.date <= ?"
			params = append(params, endDate)
		} else if len(endDate) >= 7 {
			year := endDate[0:4]
			month := endDate[5:7]
			conditions += " AND usage_statistics.date <= ?"
			params = append(params, year+"-"+month+"-31")
		}
	}
	if tokenIds != "" {
		conditions += " AND usage_statistics.token_id IN (?)"
		params = append(params, tokenIds)
	}
	if modelName != "" {
		conditions += " AND usage_statistics.model_name LIKE ?"
		params = append(params, "%"+modelName+"%")
	}
	conditions, params = appendStatsExclusionCondition(conditions, "usage_statistics.token_id", params)

	var sql string

	if groupBy == "exact" {
		sql = `
			SELECT
				SUM(total_requests) as total_requests,
				SUM(successful_requests) as successful_requests,
				SUM(failed_requests) as failed_requests,
				SUM(total_tokens) as total_tokens,
				SUM(prompt_tokens) as prompt_tokens,
				SUM(completion_tokens) as completion_tokens,
				SUM(prompt_tokens_cache) as prompt_tokens_cache,
				SUM(total_quota) as total_quota
			FROM (
				SELECT
					SUM(usage_statistics.total_requests) as total_requests,
					SUM(usage_statistics.successful_requests) as successful_requests,
					SUM(usage_statistics.failed_requests) as failed_requests,
					SUM(usage_statistics.total_tokens) as total_tokens,
					SUM(usage_statistics.prompt_tokens) as prompt_tokens,
					SUM(usage_statistics.completion_tokens) as completion_tokens,
					SUM(usage_statistics.prompt_tokens_cache) as prompt_tokens_cache,
					SUM(usage_statistics.total_quota) as total_quota
				FROM usage_statistics
				JOIN tokens ON usage_statistics.token_id = tokens.id
				WHERE 1=1` + conditions + `
				GROUP BY usage_statistics.token_id, usage_statistics.token_name
			) as grouped_data
		`
	} else {
		sql = `
			SELECT
				SUM(total_requests) as total_requests,
				SUM(successful_requests) as successful_requests,
				SUM(failed_requests) as failed_requests,
				SUM(total_tokens) as total_tokens,
				SUM(prompt_tokens) as prompt_tokens,
				SUM(completion_tokens) as completion_tokens,
				SUM(prompt_tokens_cache) as prompt_tokens_cache,
				SUM(total_quota) as total_quota
			FROM (
				SELECT
					SUM(usage_statistics.total_requests) as total_requests,
					SUM(usage_statistics.successful_requests) as successful_requests,
					SUM(usage_statistics.failed_requests) as failed_requests,
					SUM(usage_statistics.total_tokens) as total_tokens,
					SUM(usage_statistics.prompt_tokens) as prompt_tokens,
					SUM(usage_statistics.completion_tokens) as completion_tokens,
					SUM(usage_statistics.prompt_tokens_cache) as prompt_tokens_cache,
					SUM(usage_statistics.total_quota) as total_quota
				FROM usage_statistics
				JOIN tokens ON usage_statistics.token_id = tokens.id
				WHERE 1=1` + conditions + `
				GROUP BY SUBSTRING_INDEX(usage_statistics.token_name, '-', 1)
			) as grouped_data
		`
	}

	var result struct {
		TotalRequests      int `json:"total_requests"`
		SuccessfulRequests int `json:"successful_requests"`
		FailedRequests     int `json:"failed_requests"`
		TotalTokens        int `json:"total_tokens"`
		PromptTokens       int `json:"prompt_tokens"`
		CompletionTokens   int `json:"completion_tokens"`
		PromptTokensCache  int `json:"prompt_tokens_cache"`
		TotalQuota         int `json:"total_quota"`
	}

	err := db.Raw(sql, params...).Scan(&result).Error
	if err != nil {
		return nil, err
	}

	summary := map[string]interface{}{
		"total_requests":          result.TotalRequests,
		"successful_requests":     result.SuccessfulRequests,
		"failed_requests":         result.FailedRequests,
		"success_rate":            0.0,
		"total_tokens":            result.TotalTokens,
		"total_prompt_tokens":     result.PromptTokens,
		"total_completion_tokens": result.CompletionTokens,
		"total_cache_tokens":      result.PromptTokensCache,
		"total_quota":             result.TotalQuota,
	}

	if result.TotalRequests > 0 {
		summary["success_rate"] = float64(result.SuccessfulRequests) / float64(result.TotalRequests) * 100
	}

	return summary, nil
}
