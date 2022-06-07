package coralogix

import (
	"errors"
)

func getAlertByID(alertsList []interface{}, alertID string) (map[string]interface{}, error) {
	for _, alert := range alertsList {
		alert := alert.(map[string]interface{})
		if alert["unique_identifier"].(string) == alertID {
			return alert, nil
		}
	}
	return nil, errors.New("alert does not exists")
}

func flattenAlertFilter(alert interface{}) interface{} {
	alertFilter := alert.(map[string]interface{})["log_filter"].(map[string]interface{})
	// checking for keys that not allways returned
	aliasKey := ""
	if value, ok := alertFilter["alias"]; ok {
		aliasKey = value.(string)
	}
	textKey := ""
	if value, ok := alertFilter["text"]; ok {
		textKey = value.(string)
	}
	return []interface{}{map[string]interface{}{
		"text":         textKey,
		"applications": alertFilter["application_name"],
		"subsystems":   alertFilter["subsystem_name"],
		"severities":   alertFilter["severity"],
		"alias":        aliasKey,
	},
	}
}

func flattenAlertMetric(alert interface{}) interface{} {
	if alert.(map[string]interface{})["log_filter"].(map[string]interface{})["filter_type"].(string) == "metric" {
		alertCondition := alert.(map[string]interface{})["condition"].(map[string]interface{})
		// checking for keys that not allways returned
		operatorModifierKey := 0.0
		if value, ok := alertCondition["arithmetic_operator_modifier"]; ok {
			operatorModifierKey = value.(float64)
		}
		fieldKey := ""
		if value, ok := alertCondition["metric_field"]; ok {
			fieldKey = value.(string)
		}
		sourceKey := ""
		if value, ok := alertCondition["metric_source"]; ok {
			sourceKey = value.(string)
		}
		arithmeticKey := 0.0
		if value, ok := alertCondition["arithmetic_operator_modifier"]; ok {
			arithmeticKey = value.(float64)
		}
		promqlKey := ""
		if value, ok := alertCondition["promql_text"]; ok {
			promqlKey = value.(string)
		}
		return []interface{}{map[string]interface{}{
			"field":                        fieldKey,
			"source":                       sourceKey,
			"arithmetic_operator":          arithmeticKey,
			"sample_threshold_percentage":  alertCondition["sample_threshold_percentage"].(float64),
			"non_null_percentage":          alertCondition["non_null_percentage"].(float64),
			"swap_null_values":             alertCondition["swap_null_values"].(bool),
			"arithmetic_operator_modifier": operatorModifierKey,
			"promql_text":                  promqlKey,
		},
		}
	}
	return []interface{}{}
}

func flattenAlertCondition(alert interface{}) interface{} {
	alertCondition := alert.(map[string]interface{})["condition"]
	if alertCondition != nil {
		alertConditionParameters := alertCondition.(map[string]interface{})
		// checking for keys that not allways returned
		alertConditionGroupBy := ""
		if value, ok := alertConditionParameters["group_by"]; ok {
			alertConditionGroupBy = value.(string)
		}
		uniqueCountKey := ""
		if value, ok := alertConditionParameters["unique_count_key"]; ok {
			uniqueCountKey = value.(string)
		}
		relativeTimeframe := ""
		if value, ok := alertConditionParameters["relative_timeframe"]; ok {
			relativeTimeframe = value.(string)
		}
		return []interface{}{map[string]interface{}{
			"condition_type":     alertConditionParameters["condition_type"].(string),
			"threshold":          alertConditionParameters["threshold"].(float64),
			"timeframe":          alertConditionParameters["timeframe"].(string),
			"group_by":           alertConditionGroupBy,
			"unique_count_key":   uniqueCountKey,
			"relative_timeframe": relativeTimeframe,
		},
		}
	}
	return []interface{}{}
}

func flattenAlertRatio(alert interface{}) interface{} {
	alertRatio := alert.(map[string]interface{})["ratioAlerts"]
	if alertRatio != nil {
		alertRatioParameters := alertRatio.(map[string]interface{})
		// checking for keys that not allways returned
		alertRatioGroupBy := ""
		if value, ok := alertRatioParameters["group_by"]; ok {
			alertRatioGroupBy = value.([]interface{})[0].(string)
		}
		aliasKey := ""
		if value, ok := alertRatioParameters["alias"]; ok {
			aliasKey = value.(string)
		}
		return []interface{}{map[string]interface{}{
			"text":         alertRatioParameters["text"].(string),
			"applications": alertRatioParameters["application_name"],
			"subsystems":   alertRatioParameters["subsystem_name"],
			"severities":   alertRatioParameters["severity"],
			"alias":        aliasKey,
			"group_by":     alertRatioGroupBy,
		},
		}
	}
	return []interface{}{}
}

func flattenAlertNotifications(alert interface{}) interface{} {
	alertNotifications := alert.(map[string]interface{})["notifications"]
	if alertNotifications != nil {
		alertNotificationsParameters := alertNotifications.(map[string]interface{})
		return []interface{}{map[string]interface{}{
			"emails":       alertNotificationsParameters["emails"],
			"integrations": alertNotificationsParameters["integrations"],
		},
		}
	}
	return []interface{}{}
}

func flattenAlertSchedule(alert interface{}) interface{} {
	alertSchedule := alert.(map[string]interface{})["active_when"]
	if alertSchedule != nil {
		if len(alertSchedule.(map[string]interface{})["timeframes"].([]interface{})) == 0 {
			return []interface{}{}
		}
		alertScheduleParameters := alertSchedule.(map[string]interface{})["timeframes"].([]interface{})[0].(map[string]interface{})
		return []interface{}{map[string]interface{}{
			"days":  transformWeekListReverse(alertScheduleParameters["days_of_week"].([]interface{})),
			"start": alertScheduleParameters["activity_starts"],
			"end":   alertScheduleParameters["activity_ends"],
		},
		}
	}
	return []interface{}{}
}
func flattenRules(rules []interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(rules))
	for _, rule := range rules {
		rule := rule.(map[string]interface{})
		l := map[string]interface{}{
			"id":                rule["id"].(string),
			"name":              rule["name"].(string),
			"type":              rule["type"].(string),
			"description":       rule["description"].(string),
			"order":             rule["order"].(float64),
			"enabled":           rule["enabled"].(bool),
			"rule_matcher":      flattenRuleMatchers(rule["ruleMatchers"].([]interface{})),
			"expression":        rule["rule"].(string),
			"source_field":      rule["sourceField"].(string),
			"destination_field": rule["destinationField"].(string),
			"replace_value":     rule["replaceNewVal"].(string),
		}
		result = append(result, l)
	}
	return result
}

func flattenRuleMatchers(ruleMatchers []interface{}) []map[string]interface{} {
	if len(ruleMatchers) > 0 {
		result := make([]map[string]interface{}, 0, len(ruleMatchers))
		for _, ruleMatcher := range ruleMatchers {
			ruleMatcher := ruleMatcher.(map[string]interface{})
			l := map[string]interface{}{
				"field":      ruleMatcher["field"],
				"constraint": ruleMatcher["constraint"],
			}
			result = append(result, l)
		}
		return result
	}
	return nil
}

func transformWeekList(days []interface{}) []int {
	week := make([]int, 0, len(days))
	week_days := map[string]int{"Mo": 0, "Tu": 1, "We": 2, "Th": 3, "Fr": 4, "Sa": 5, "Su": 6}
	for _, day := range days {
		week = append(week, week_days[day.(string)])
	}
	return week
}

func transformWeekListReverse(days []interface{}) []string {
	week := make([]string, 0, len(days))
	week_days := map[float64]string{0: "Mo", 1: "Tu", 2: "We", 3: "Th", 4: "Fr", 5: "Sa", 6: "Su"}
	for _, day := range days {
		week = append(week, week_days[day.(float64)])
	}
	return week
}

func getFirstOrNil(list []interface{}) interface{} {
	if len(list) > 0 {
		return list[0]
	}
	return nil
}
