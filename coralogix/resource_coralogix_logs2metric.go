package coralogix

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"time"

	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	logs2metricv2 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/logs2metrics/v2"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var validSeverities = maps.Keys(logs2metricv2.Severity_value)

func resourceCoralogixLogs2Metric() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCoralogixLogs2MetricCreate,
		ReadContext:   resourceCoralogixLogs2MetricRead,
		UpdateContext: resourceCoralogixLogs2MetricUpdate,
		DeleteContext: resourceCoralogixLogs2MetricDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Minute),
			Read:   schema.DefaultTimeout(30 * time.Minute),
			Update: schema.DefaultTimeout(60 * time.Minute),
			Delete: schema.DefaultTimeout(30 * time.Minute),
		},

		Schema: Logs2MetricSchema(),
	}
}

func resourceCoralogixLogs2MetricCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	log2Metric, err := extractLogs2Metric(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log2MetricReq := &logs2metricv2.CreateL2MRequest{
		L2M: log2Metric,
	}

	log.Printf("[INFO] Creating new logs2metric: %#v", log2MetricReq)
	Logs2MetricResp, err := meta.(*clientset.ClientSet).Logs2Metrics().CreateLogs2Metric(ctx, log2MetricReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err)
	}
	log.Printf("[INFO] Submitted new logs2metric: %#v", Logs2MetricResp)
	d.SetId(Logs2MetricResp.GetId().GetValue())

	return resourceCoralogixLogs2MetricRead(ctx, d, meta)
}

func resourceCoralogixLogs2MetricRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	getLogs2MetricRequest := &logs2metricv2.GetL2MRequest{
		Id: wrapperspb.String(id),
	}

	log.Printf("[INFO] Reading logs2metric %s", id)
	logs2MetricResp, err := meta.(*clientset.ClientSet).Logs2Metrics().GetLogs2Metric(ctx, getLogs2MetricRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "logs2metric", id)
	}
	log.Printf("[INFO] Received logs2metric: %#v", logs2MetricResp)

	return setLogs2Metric(d, logs2MetricResp)
}

func resourceCoralogixLogs2MetricUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	req, err := extractLogs2Metric(d)
	if err != nil {
		return diag.FromErr(err)
	}

	req.Id = wrapperspb.String(d.Id())
	updateLogs2MetricRequest := &logs2metricv2.ReplaceL2MRequest{
		L2M: req,
	}

	log.Printf("[INFO] Updating logs2metric %s", updateLogs2MetricRequest)
	log2MetricResp, err := meta.(*clientset.ClientSet).Logs2Metrics().UpdateLogs2Metric(ctx, updateLogs2MetricRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "logs2metric", req.Id.GetValue())
	}
	log.Printf("[INFO] Submitted updated logs2metric: %#v", log2MetricResp)

	return resourceCoralogixLogs2MetricRead(ctx, d, meta)
}

func resourceCoralogixLogs2MetricDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	deleteLogs2MetricRequest := &logs2metricv2.DeleteL2MRequest{
		Id: wrapperspb.String(id),
	}

	log.Printf("[INFO] Deleting logs2metric %s\n", id)
	_, err := meta.(*clientset.ClientSet).Logs2Metrics().DeleteLogs2Metric(ctx, deleteLogs2MetricRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v\n", err)
		return handleRpcErrorWithID(err, "logs2metric", id)
	}
	log.Printf("[INFO] logs2metric %s deleted\n", id)

	d.SetId("")
	return nil
}

func extractLogs2Metric(d *schema.ResourceData) (*logs2metricv2.L2M, error) {
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))
	query := expandQuery(d.Get("query"))
	permutations := expandPermutations(d.Get("permutations"))
	fields := expandFields(d.Get("metric_fields"))
	labels := expandLabels(d.Get("metric_labels"))

	return &logs2metricv2.L2M{
		Name:         name,
		Description:  description,
		Query:        query,
		Permutations: permutations,
		MetricFields: fields,
		MetricLabels: labels,
	}, nil
}

func expandQuery(v interface{}) *logs2metricv2.LogsQuery {
	l := v.([]interface{})
	if len(l) == 0 || l[0] == nil {
		return &logs2metricv2.LogsQuery{}
	}
	raw := l[0]
	m := raw.(map[string]interface{})

	searchQuery := wrapperspb.String(m["lucene"].(string))
	applications := interfaceSliceToWrappedStringSlice(m["applications"].(*schema.Set).List())
	subsystems := interfaceSliceToWrappedStringSlice(m["subsystems"].(*schema.Set).List())
	severities := expandSeverities(m["severities"].(*schema.Set).List())

	return &logs2metricv2.LogsQuery{
		Lucene:                 searchQuery,
		ApplicationnameFilters: applications,
		SubsystemnameFilters:   subsystems,
		SeverityFilters:        severities,
	}
}

func expandSeverities(severities []interface{}) []logs2metricv2.Severity {
	result := make([]logs2metricv2.Severity, 0, len(severities))
	for _, s := range severities {
		severity := logs2metricv2.Severity(logs2metricv2.Severity_value[s.(string)])
		result = append(result, severity)
	}

	return result
}

func expandLabels(v interface{}) []*logs2metricv2.MetricLabel {
	labels := v.(*schema.Set).List()
	result := make([]*logs2metricv2.MetricLabel, 0, len(labels))
	for _, l := range labels {
		label := expandLabel(l)
		result = append(result, label)
	}

	return result
}

func expandLabel(v interface{}) *logs2metricv2.MetricLabel {
	m := v.(map[string]interface{})
	targetLabel := wrapperspb.String(m["target_label"].(string))
	sourceField := wrapperspb.String(m["source_field"].(string))
	return &logs2metricv2.MetricLabel{
		TargetLabel: targetLabel,
		SourceField: sourceField,
	}
}

func expandFields(v interface{}) []*logs2metricv2.MetricField {
	v = v.(*schema.Set).List()
	fields := v.([]interface{})
	result := make([]*logs2metricv2.MetricField, 0, len(fields))
	for _, f := range fields {
		field := expandField(f)
		result = append(result, field)
	}

	return result
}

func expandField(v interface{}) *logs2metricv2.MetricField {
	m := v.(map[string]interface{})
	targetBaseMetricName := wrapperspb.String(m["target_base_metric_name"].(string))
	sourceField := wrapperspb.String(m["source_field"].(string))
	return &logs2metricv2.MetricField{
		TargetBaseMetricName: targetBaseMetricName,
		SourceField:          sourceField,
	}
}

func expandPermutations(v interface{}) *logs2metricv2.L2MPermutations {
	l := v.([]interface{})
	if len(l) == 0 || l[0] == nil {
		return &logs2metricv2.L2MPermutations{}
	}
	raw := l[0]
	m := raw.(map[string]interface{})
	limit := int32(m["limit"].(int))
	hasExceededLimit := m["has_exceed_limit"].(bool)
	return &logs2metricv2.L2MPermutations{
		Limit:            limit,
		HasExceededLimit: hasExceededLimit,
	}
}

func setLogs2Metric(d *schema.ResourceData, logs2Metric *logs2metricv2.L2M) diag.Diagnostics {
	if err := d.Set("name", logs2Metric.GetName().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("description", logs2Metric.GetDescription().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("query", flattenQuery(logs2Metric.GetQuery())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("metric_fields", flattenMetricFields(logs2Metric.GetMetricFields())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("metric_labels", flattenMetricLabels(logs2Metric.GetMetricLabels())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("permutations", flattenPermutations(logs2Metric.GetPermutations())); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func flattenMetricFields(fields []*logs2metricv2.MetricField) interface{} {
	transformed := schema.NewSet(metricFieldsHash(), []interface{}{})
	for _, f := range fields {
		field := flattenMetricField(f)
		transformed.Add(field)
	}
	return transformed
}

func metricFieldsHash() schema.SchemaSetFunc {
	return schema.HashResource(metricFields())
}

func metricFields() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"target_base_metric_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"source_field": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func flattenMetricField(field *logs2metricv2.MetricField) interface{} {
	return map[string]interface{}{
		"target_base_metric_name": field.GetTargetBaseMetricName().GetValue(),
		"source_field":            field.GetSourceField().GetValue(),
	}
}

func metricLabels() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"target_label": {
				Type:     schema.TypeString,
				Required: true,
			},
			"source_field": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func metricLabelsHash() schema.SchemaSetFunc {
	return schema.HashResource(metricLabels())
}

func flattenMetricLabels(labels []*logs2metricv2.MetricLabel) interface{} {
	result := make([]interface{}, 0, len(labels))
	for _, l := range labels {
		label := flattenMetricLabel(l)
		result = append(result, label)
	}
	return result
}

func flattenMetricLabel(label *logs2metricv2.MetricLabel) interface{} {
	return map[string]interface{}{
		"target_label": label.GetTargetLabel().GetValue(),
		"source_field": label.GetSourceField().GetValue(),
	}
}

func flattenPermutations(permutations *logs2metricv2.L2MPermutations) interface{} {
	return []interface{}{map[string]interface{}{
		"limit":            permutations.GetLimit(),
		"has_exceed_limit": permutations.GetHasExceededLimit(),
	},
	}
}

func flattenQuery(query *logs2metricv2.LogsQuery) interface{} {
	m := make(map[string]interface{})

	lucene := query.GetLucene().GetValue()
	if lucene != "" {
		m["lucene"] = lucene
	}

	applications := query.GetApplicationnameFilters()
	if len(applications) > 0 {
		m["applications"] = wrappedStringSliceToStringSlice(applications)
	}

	subsystems := query.GetSubsystemnameFilters()
	if len(subsystems) > 0 {
		m["subsystems"] = wrappedStringSliceToStringSlice(subsystems)
	}

	severities := flattenSeverities(query.GetSeverityFilters())
	if len(severities) > 0 {
		m["severities"] = severities
	}

	return []interface{}{m}
}

func flattenSeverities(severities []logs2metricv2.Severity) []string {
	result := make([]string, 0, len(severities))
	for _, s := range severities {
		result = append(result, logs2metricv2.Severity_name[int32(s)])
	}
	return result
}

func Logs2MetricSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Type:     schema.TypeString,
			Required: true,
			ValidateFunc: validation.All(
				validation.StringMatch(regexp.MustCompile(`^[A-Za-z\d_:-]*$`), "Invalid metric name, name may only contain ASCII letters and digits, as well as underscores and colons."),
				validation.StringIsNotEmpty,
			),
			Description: "Log2Metric name. Log2Metric names have to be unique per account.",
		},
		"description": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Log2Metric description.",
		},
		"query": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"lucene": {
						Type:         schema.TypeString,
						Optional:     true,
						ValidateFunc: validation.StringIsValidRegExp,
						Description:  "The search_query that we wanted to be notified on.",
					},
					"applications": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
						Description: "An array that contains log’s application names that we want to be alerted on.",
						Set:         schema.HashString,
					},
					"subsystems": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
						Description: "An array that contains log’s subsystem names that we want to be notified on.",
						Set:         schema.HashString,
					},
					"severities": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Schema{
							Type:         schema.TypeString,
							ValidateFunc: validation.StringInSlice(validSeverities, false),
						},
						Set:         schema.HashString,
						Description: fmt.Sprintf("An array of severities that we interested in. Can be one of %q", validSeverities),
					},
				},
			},
		},
		"metric_fields": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem:     metricFields(),
			Set:      metricFieldsHash(),
		},
		"metric_labels": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem:     metricLabels(),
			Set:      metricLabelsHash(),
		},
		"permutations": {
			Type:     schema.TypeList,
			MaxItems: 1,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"limit": {
						Type:         schema.TypeInt,
						Required:     true,
						ValidateFunc: validation.IntAtLeast(0),
					},
					"has_exceed_limit": {
						Type:     schema.TypeBool,
						Computed: true,
					},
				},
			},
		},
	}
}
