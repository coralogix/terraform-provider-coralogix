package coralogix

import (
	"errors"
)

func getAlertByID(alertsList []interface{}, alertID string) (map[string]interface{}, error) {
	for _, alert := range alertsList {
		alert := alert.(map[string]interface{})
		if alert["id"].(string) == alertID {
			return alert, nil
		}
	}
	return nil, errors.New("alert is not exists")
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

func flattenRules(rules []interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(rules))
	for _, rule := range rules {
		rule := rule.(map[string]interface{})
		l := map[string]interface{}{
			"id":                rule["Id"].(string),
			"name":              rule["Name"].(string),
			"type":              rule["Type"].(string),
			"description":       rule["Description"].(string),
			"order":             rule["Order"].(float64),
			"enabled":           rule["Enabled"].(bool),
			"rule_matcher":      flattenRuleMatchers(rule["RuleMatchers"].([]interface{})),
			"expression":        rule["Rule"].(string),
			"source_field":      rule["SourceField"].(string),
			"destination_field": rule["DestinationField"].(string),
			"replace_value":     rule["ReplaceNewVal"].(string),
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

func getFirstOrNil(list []interface{}) interface{} {
	if len(list) > 0 {
		return list[0]
	}
	return nil
}
