package coralogix

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
		if value == nil {
			textKey = ""
		} else {
			textKey = value.(string)
		}
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
		if value, ok := alertCondition["arithmetic_operator"]; ok {
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

func flattenAlertCondition(alert interface{}, group_by_array_flag bool) interface{} {
	alertCondition := alert.(map[string]interface{})["condition"]
	if alertCondition != nil {
		alertConditionParameters := alertCondition.(map[string]interface{})
		// a check for group_by_array and group_by. will be changed when we remove group_by
		alertConditionGroupBy := ""
		alertConditionGroupByArray := make([]string, 0, 2)
		if group_by_array_flag {
			// use group_by_array key
			if value, ok := alertConditionParameters["group_by"]; ok {
				alertConditionGroupByArray = append(alertConditionGroupByArray, value.(string))
				index := 2
				for {
					key := fmt.Sprintf("group_by_lvl%d", index)
					if value, ok := alertConditionParameters[key]; ok {
						alertConditionGroupByArray = append(alertConditionGroupByArray, value.(string))
						index++
					} else {
						break
					}
				}
			}
		} else {
			if value, ok := alertConditionParameters["group_by"]; ok {
				alertConditionGroupBy = value.(string)
			}
		}
		// checking for keys that not allways returned
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
			"group_by_array":     alertConditionGroupByArray,
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
func flattenRules(rulesGroup []interface{}) []interface{} {
	result := make([]interface{}, 0, len(rulesGroup))
	for _, group := range rulesGroup {
		group := group.(map[string]interface{})
		rules := make([]map[string]interface{}, 0, len(group))
		for _, rule := range group {
			rule := rule.(map[string]interface{})
			r := map[string]interface{}{
				"id":                   rule["id"].(string),
				"name":                 rule["name"].(string),
				"description":          rule["description"].(string),
				"enabled":              rule["enabled"].(bool),
				"rule_matcher":         flattenRuleMatchers(rule["ruleMatchers"]),
				"expression":           rule["rule"].(string),
				"source_field":         rule["sourceField"].(string),
				"type":                 rule["type"].(string),
				"order":                rule["order"].(float64),
				"keep_blocked_logs":    rule["keepBlockedLogs"].(bool),
				"delete_source":        rule["deleteSource"].(bool),
				"escaped_value":        rule["escapedValue"].(bool),
				"overwrite_destinaton": rule["overrideDest"].(bool),
			}
			// rule type timestampextract needs and return different fields
			if rule["type"] == "timestampextract" {
				r["destination_field"] = "text"
				r["replace_value"] = ""
				r["format_standard"] = "formatStandard"
				r["time_format"] = "timeFormat"
			} else {
				r["destination_field"] = rule["destinationField"].(string)
				r["replace_value"] = rule["replaceNewVal"].(string)
			}
			rules = append(rules, r)
		}
		l := map[string]interface{}{
			"group": rules,
		}
		result = append(result, l)
	}
	return result
}

func flattenRuleMatchers(ruleMatchers interface{}) []map[string]interface{} {
	if ruleMatchers == nil {
		return nil
	}
	ruleMatchersArr := ruleMatchers.([]interface{})
	if len(ruleMatchersArr) > 0 {
		result := make([]map[string]interface{}, 0, len(ruleMatchersArr))
		for _, ruleMatcher := range ruleMatchersArr {
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

// returns the timeframe chose in seconds
func getTimeframeInSeconds(time string) int {
	timeMap := map[string]int{"5Min": 300, "10Min": 600, "20Min": 1200, "30Min": 1800, "1H": 3600, "2H": 7200, "3H": 10800, "4H": 14400, "6H": 21600, "12H": 43200, "24H": 86400, "HOUR": 3600, "DAY": 86400}
	return timeMap[time]
}

// resource values validation on create or update,
// returns error or nil
func valuesValidation(d *schema.ResourceData) error {
	alertType := d.Get("type").(string)
	filter := getFirstOrNil(d.Get("filter").(*schema.Set).List())
	condition := getFirstOrNil(d.Get("condition").(*schema.Set).List())
	ratio := getFirstOrNil(d.Get("ratio").(*schema.Set).List())
	metric := getFirstOrNil(d.Get("metric").(*schema.Set).List())
	schedule := getFirstOrNil(d.Get("schedule").(*schema.Set).List())
	if condition == nil {
		if alertType != "text" {
			return fmt.Errorf("alert of type %s must have condition block", alertType)
		}
	}
	// conditions affecting multiple alertTypes but not all are copied for simplicity
	switch alertType {
	case "text":
		if condition != nil {
			if condition.(map[string]interface{})["condition_type"] == "new_value" {
				if condition.(map[string]interface{})["group_by"] == "" && len(condition.(map[string]interface{})["group_by_array"].(*schema.Set).List()) == 0 {
					return errors.New("when alert condition is of type 'new_value' condition.group_by_array should be defined")
				}
				if len(condition.(map[string]interface{})["group_by_array"].(*schema.Set).List()) > 1 {
					return errors.New("when alert condition is of type 'new_value' condition.group_by_array cannot be more than one element")
				}
				timeMapNewValue := map[string]bool{"12H": true, "24H": true, "48H": true, "72H": true, "1W": true, "1M": true, "2M": true, "3M": true}
				if _, ok := timeMapNewValue[condition.(map[string]interface{})["timeframe"].(string)]; !ok {
					return fmt.Errorf("timeframe has to match '%s' alert values", alertType)
				}
			} else {
				timeMapBasic := map[string]bool{"5MIN": true, "10MIN": true, "20MIN": true, "30MIN": true, "1H": true, "2H": true, "3H": true, "4H": true, "6H": true, "12H": true, "24H": true}
				if _, ok := timeMapBasic[condition.(map[string]interface{})["timeframe"].(string)]; !ok {
					return fmt.Errorf("timeframe has to match '%s' alert values", alertType)
				}
			}
			if condition.(map[string]interface{})["unique_count_key"] != "" {
				return errors.New("when alert is of type 'text' condition.unique_count_key should not be defined")
			}
		}
		if filter.(map[string]interface{})["alias"].(string) != "" {
			return fmt.Errorf("alerts of type '%s' cannot define filter.alias", alertType)
		}
	case "unique_count":
		if condition.(map[string]interface{})["unique_count_key"] == "" {
			return errors.New("when alert is of type 'unique_count' condition.unique_count_key should be defined")
		}
		if condition.(map[string]interface{})["condition_type"] != "more_than" {
			return errors.New("when alert is of type 'unique_count' condition.condition_type should be 'more_than'")
		}
		timeMapBasic := map[string]bool{"5MIN": true, "10MIN": true, "20MIN": true, "30MIN": true, "1H": true, "2H": true, "3H": true, "4H": true, "6H": true, "12H": true, "24H": true}
		if _, ok := timeMapBasic[condition.(map[string]interface{})["timeframe"].(string)]; !ok {
			return fmt.Errorf("timeframe has to match '%s' alert values", alertType)
		}
		if filter.(map[string]interface{})["alias"].(string) != "" {
			return fmt.Errorf("alerts of type '%s' cannot define filter.alias", alertType)
		}
	case "metric":
		if metric == nil {
			return errors.New("alert of type 'metric' must have metric block")
		}
		if metric.(map[string]interface{})["promql_text"] != "" {
			if condition.(map[string]interface{})["group_by"] != "" || len(condition.(map[string]interface{})["group_by_array"].(*schema.Set).List()) != 0 ||
				filter.(map[string]interface{})["text"] != "" || metric.(map[string]interface{})["field"] != "" || metric.(map[string]interface{})["source"] != "" ||
				metric.(map[string]interface{})["arithmetic_operator"] != 0 {
				return errors.New("alert of type metric with promql_text must not define these fields: [metric.field, metric.source, metric.arithmetic_operator," +
					" filter.text, condition.group_by, condition.group_by_array]")
			}
		} else {
			if metric.(map[string]interface{})["field"] == "" || metric.(map[string]interface{})["source"] == "" {
				return errors.New("alert of type metric without promql_text must define these fields: [metric.field, metric.source]")
			}
		}
		if getFirstOrNil(filter.(map[string]interface{})["severities"].(*schema.Set).List()) != nil {
			return errors.New("alert of type metric cannot define filter.severities")
		}
		if getFirstOrNil(filter.(map[string]interface{})["applications"].(*schema.Set).List()) != nil {
			return errors.New("alert of type metric cannot define filter.applications")
		}
		if getFirstOrNil(filter.(map[string]interface{})["subsystems"].(*schema.Set).List()) != nil {
			return errors.New("alert of type metric cannot define filter.subsystems")
		}
		if metric.(map[string]interface{})["arithmetic_operator"] != 5 && metric.(map[string]interface{})["arithmetic_operator_modifier"] != 0 {
			return errors.New("alert of type metric cannot define metric.arithmetic_operator_modifier when metric.arithmetic_operator is not 5 (percentile)")
		}
		if filter.(map[string]interface{})["alias"].(string) != "" {
			return errors.New("alert of type metric cannot define filter.alias")
		}
		if condition.(map[string]interface{})["condition_type"] != "more_than" && condition.(map[string]interface{})["condition_type"] != "less_than" {
			return errors.New("condition.condition_type has to match metric alert values")
		}
		if condition.(map[string]interface{})["unique_count_key"] != "" {
			return errors.New("when alert is of type 'metric' condition.unique_count_key should not be defined")
		}
		timeMapBasic := map[string]bool{"5MIN": true, "10MIN": true, "20MIN": true, "30MIN": true, "1H": true, "2H": true, "3H": true, "4H": true, "6H": true, "12H": true, "24H": true}
		if _, ok := timeMapBasic[condition.(map[string]interface{})["timeframe"].(string)]; !ok {
			return fmt.Errorf("timeframe has to match '%s' alert values", alertType)
		}
		if filter.(map[string]interface{})["alias"].(string) != "" {
			return fmt.Errorf("alerts of type '%s' cannot define filter.alias", alertType)
		}
	case "relative_time":
		if condition.(map[string]interface{})["condition_type"] != "more_than" && condition.(map[string]interface{})["condition_type"] != "less_than" {
			return errors.New("condition.condition_type has to match relative_time alert values")
		}
		timeMap := map[string]bool{"HOUR": true, "DAY": true}
		if _, ok := timeMap[condition.(map[string]interface{})["timeframe"].(string)]; !ok {
			return fmt.Errorf("timeframe has to match '%s' alert values", alertType)
		}
		timeMapRelative := map[string]bool{"HOUR": true, "DAY": true, "WEEK": true, "MONTH": true}
		if _, ok := timeMapRelative[condition.(map[string]interface{})["relative_timeframe"].(string)]; !ok {
			return fmt.Errorf("relative timeframe has to match '%s' alert values", alertType)
		}
		if filter.(map[string]interface{})["alias"].(string) != "" {
			return fmt.Errorf("alerts of type '%s' cannot define filter.alias", alertType)
		}
		if condition.(map[string]interface{})["unique_count_key"] != "" {
			return errors.New("when alert is of type 'relative_time' condition.unique_count_key should not be defined")
		}
	case "ratio":
		if ratio == nil {
			return errors.New("alert of type 'ratio' must have ratio block")
		}
		if filter.(map[string]interface{})["alias"].(string) == "" {
			return errors.New("alert of type 'ratio' must have filter.alias defined")
		}
		if condition.(map[string]interface{})["condition_type"] != "more_than" && condition.(map[string]interface{})["condition_type"] != "less_than" {
			return errors.New("condition.condition_type has to match 'ratio' alert values")
		}
		timeMapBasic := map[string]bool{"5MIN": true, "10MIN": true, "20MIN": true, "30MIN": true, "1H": true, "2H": true, "3H": true, "4H": true, "6H": true, "12H": true, "24H": true}
		if _, ok := timeMapBasic[condition.(map[string]interface{})["timeframe"].(string)]; !ok {
			return fmt.Errorf("timeframe has to match '%s' alert values", alertType)
		}
		if condition.(map[string]interface{})["unique_count_key"] != "" {
			return errors.New("when alert is of type 'ratio' condition.unique_count_key should not be defined")
		}
	}
	// non-specific checks
	if schedule != nil {
		r, _ := regexp.Compile(`[0-9]{2}:[0-9]{2}:[0-9]{2}`)
		if ok := r.MatchString(schedule.(map[string]interface{})["start"].(string)); !ok {
			return errors.New("schedule.start must be in format HH:MM:SS")
		}
		if ok := r.MatchString(schedule.(map[string]interface{})["end"].(string)); !ok {
			return errors.New("schedule.end must be in format HH:MM:SS")
		}
	}
	if condition != nil {
		if condition.(map[string]interface{})["condition_type"] == "less_than" {
			if condition.(map[string]interface{})["group_by"] != "" || len(condition.(map[string]interface{})["group_by_array"].(*schema.Set).List()) != 0 {
				return errors.New("when alert condition is of type 'less_than', condition.group_by_array and condition.group_by should not be defined")
			}
			if timeInSeconds := getTimeframeInSeconds(condition.(map[string]interface{})["timeframe"].(string)); d.Get("notify_every").(int) < timeInSeconds {
				return fmt.Errorf("when alert condition is of type 'less_than', notify_every has to be as long as condition.timeframe, atleast %d", timeInSeconds)
			}
		}
		if condition.(map[string]interface{})["group_by"] != "" && len(condition.(map[string]interface{})["group_by_array"].(*schema.Set).List()) != 0 {
			return errors.New("when condition.group_by_array is defined, condition.group_by cannot be defined")
		}
	}
	return nil
}
func ruleValuesValidation(d *schema.ResourceData) error {
	ruleType := d.Get("type").(string)
	switch ruleType {
	case "extract":
		if value := d.Get("replace_value").(string); value != "" {
			return fmt.Errorf("rules of type '%s' cannot define replace_value", ruleType)
		}
		if value := d.Get("destination_field").(string); value != "text" {
			return fmt.Errorf("rules of type '%s' cannot define destination_field", ruleType)
		}
	case "jsonextract":
		if value := d.Get("replace_value").(string); value != "" {
			return fmt.Errorf("rules of type '%s' cannot define replace_value", ruleType)
		}
		if value := d.Get("source_field").(string); value != "text" {
			return fmt.Errorf("rules of type '%s' cannot define source_field, to define the field to exract define 'expression'", ruleType)
		}
	case "parse":
		if value := d.Get("replace_value").(string); value != "" {
			return fmt.Errorf("rules of type '%s' cannot define replace_value", ruleType)
		}
	case "replace":
	case "timestampextract":
		if value := d.Get("expression").(string); value != ".*" {
			return fmt.Errorf("rules of type '%s' cannot define expression", ruleType)
		}
		if value := d.Get("destination_field").(string); value != "text" {
			return fmt.Errorf("rules of type '%s' cannot define destination_field", ruleType)
		}
		if value := d.Get("format_standard").(string); value == "" {
			return fmt.Errorf("rules of type '%s' must define format_standard", ruleType)
		}
		if value := d.Get("time_format").(string); value == "" {
			return fmt.Errorf("rules of type '%s' must define time_format", ruleType)
		}
	case "removefields":
		if value := d.Get("expression").(string); value == "" {
			return fmt.Errorf("rules of type '%s' cannot set expression as empty", ruleType)
		}
	case "block":
		if value := d.Get("replace_value").(string); value != "" {
			return fmt.Errorf("rules of type '%s' cannot define replace_value", ruleType)
		}
		if value := d.Get("destination_field").(string); value != "text" {
			return fmt.Errorf("rules of type '%s' cannot define destination_field", ruleType)
		}
	case "allow":
		if value := d.Get("replace_value").(string); value != "" {
			return fmt.Errorf("rules of type '%s' cannot define replace_value", ruleType)
		}
		if value := d.Get("destination_field").(string); value != "text" {
			return fmt.Errorf("rules of type '%s' cannot define destination_field", ruleType)
		}
	case "jsonstringify":
		if value := d.Get("expression").(string); value != ".*" {
			return fmt.Errorf("rules of type '%s' cannot define expression", ruleType)
		}
	case "jsonparse":
		if value := d.Get("expression").(string); value != ".*" {
			return fmt.Errorf("rules of type '%s' cannot define expression", ruleType)
		}
	}
	return nil
}
