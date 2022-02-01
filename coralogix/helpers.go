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
	return nil, errors.New("Alert is not exists")
}

func flattenAlertFilter(alert interface{}) interface{} {
	alertFilter := alert.(map[string]interface{})["log_filter"].(map[string]interface{})
	return map[string]interface{}{
		"text":         alertFilter["text"].(string),
		"applications": alertFilter["application_name"],
		"subsystems":   alertFilter["subsystem_name"],
		"severities":   alertFilter["severity"],
	}
}

func flattenAlertMetric(alert interface{}) interface{} {
	if alert.(map[string]interface{})["log_filter"].(map[string]interface{})["filter_type"].(string) == "metric" {
		alertCondition := alert.(map[string]interface{})["condition"].(map[string]interface{})
		return map[string]interface{}{
			"field":                        alertCondition["text"].(string),
			"source":                       alertCondition["application_name"].(string),
			"arithmetic_operator":          alertCondition["arithmetic_operator"].(float64),
			"arithmetic_operator_modifier": alertCondition["arithmetic_operator_modifier"].(float64),
			"sample_threshold_percentage":  alertCondition["sample_threshold_percentage"].(float64),
			"non_null_percentage":          alertCondition["non_null_percentage"].(float64),
			"swap_null_values":             alertCondition["swap_null_values"].(bool),
		}
	}
	return nil
}

func flattenAlertCondition(alert interface{}) interface{} {
	alertCondition := alert.(map[string]interface{})["condition"]
	if alertCondition != nil {
		alertConditionParameters := alertCondition.(map[string]interface{})

		alertConditionGroupBy, found := alertConditionParameters["group_by"]
		if !found {
			alertConditionGroupBy = "none"
		} else {
			alertConditionGroupBy = alertConditionParameters["group_by"].(string)
		}

		return map[string]interface{}{
			"type":      alertConditionParameters["condition_type"].(string),
			"threshold": alertConditionParameters["threshold"].(float64),
			"timeframe": alertConditionParameters["timeframe"].(string),
			"group_by":  alertConditionGroupBy,
		}
	}
	return map[string]interface{}{
		"type": "immediately",
	}
}

func flattenAlertNotifications(alert interface{}) interface{} {
	alertNotifications := alert.(map[string]interface{})["notifications"]
	if alertNotifications != nil {
		alertNotificationsParameters := alertNotifications.(map[string]interface{})
		return map[string]interface{}{
			"emails":       alertNotificationsParameters["emails"],
			"integrations": alertNotificationsParameters["integrations"],
		}
	}
	return "disabled"
}

func flattenAlertSchedule(alert interface{}) interface{} {
	alertSchedule := alert.(map[string]interface{})["active_when"].(map[string]interface{})["timeframes"].([]interface{})[0].(map[string]interface{})
	return map[string]interface{}{
		"days":  transformWeekListReverse(alertSchedule["days_of_week"].([]interface{})),
		"start": alertSchedule["activity_starts"],
		"end":   alertSchedule["activity_ends"],
	}
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
