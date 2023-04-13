package coralogix

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"time"

	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	e2m "terraform-provider-coralogix/coralogix/clientset/grpc/events2metrics/v2"
	l2m "terraform-provider-coralogix/coralogix/clientset/grpc/logs2metrics/v2"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var validSeverities = getKeysInt32(l2m.Severity_value)

func resourceCoralogixEvents2Metric() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCoralogixEvents2MetricCreate,
		ReadContext:   resourceCoralogixEvents2MetricRead,
		UpdateContext: resourceCoralogixEvents2MetricUpdate,
		DeleteContext: resourceCoralogixEvents2MetricDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Second),
			Read:   schema.DefaultTimeout(30 * time.Second),
			Update: schema.DefaultTimeout(60 * time.Second),
			Delete: schema.DefaultTimeout(30 * time.Second),
		},

		Schema: Events2MetricSchema(),
	}
}

func resourceCoralogixEvents2MetricCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	e2mCreateReq := extractCreateE2M(d)

	log.Printf("[INFO] Creating new Events2metric: %#v", *e2mCreateReq)
	Events2MetricResp, err := meta.(*clientset.ClientSet).Events2Metrics().CreateEvents2Metric(ctx, e2mCreateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "Events2metric")
	}
	log.Printf("[INFO] Submitted new Events2metric: %#v", Events2MetricResp)
	d.SetId(Events2MetricResp.GetE2M().GetId().GetValue())

	return resourceCoralogixEvents2MetricRead(ctx, d, meta)
}

func resourceCoralogixEvents2MetricRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	getE2MRequest := &e2m.GetE2MRequest{
		Id: wrapperspb.String(id),
	}

	log.Printf("[INFO] Reading Events2metric %s", id)
	getE2MResp, err := meta.(*clientset.ClientSet).Events2Metrics().GetEvents2Metric(ctx, getE2MRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "Events2metric", id)
	}
	log.Printf("[INFO] Received Events2metric: %#v", getE2MResp)

	return setEvents2Metric(d, getE2MResp.GetE2M())
}

func setEvents2Metric(d *schema.ResourceData, events2Metric *e2m.E2M) diag.Diagnostics {
	if err := d.Set("name", events2Metric.GetName().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("description", events2Metric.GetDescription().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("permutations", flattenE2MPermutations(events2Metric.GetPermutations())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("metric_labels", flattenE2MMetricLabels(events2Metric.GetMetricLabels())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("metric_fields", flattenE2MMetricFields(events2Metric.GetMetricFields())); err != nil {
		return diag.FromErr(err)
	}

	switch e2mType := events2Metric.GetQuery().(type) {
	case *e2m.E2M_SpansQuery:
		if err := d.Set("spans_query", flattenSpansQuery(e2mType.SpansQuery)); err != nil {
			return diag.FromErr(err)
		}
	case *e2m.E2M_LogsQuery:
		if err := d.Set("logs_query", flattenLogQuery(e2mType.LogsQuery)); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceCoralogixEvents2MetricUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	updateE2MRequest := extractUpdateEvents2Metric(d)

	log.Printf("[INFO] Updating Events2metric %s", updateE2MRequest)
	updateE2MResp, err := meta.(*clientset.ClientSet).Events2Metrics().UpdateEventsMetric(ctx, updateE2MRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "Events2metric", updateE2MRequest.E2M.GetId().GetValue())
	}
	log.Printf("[INFO] Submitted updated Events2metric: %#v", updateE2MResp)

	return resourceCoralogixEvents2MetricRead(ctx, d, meta)
}

func resourceCoralogixEvents2MetricDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	deleteEvents2MetricRequest := &e2m.DeleteE2MRequest{
		Id: wrapperspb.String(id),
	}

	log.Printf("[INFO] Deleting Events2metric %s\n", id)
	_, err := meta.(*clientset.ClientSet).Events2Metrics().DeleteEvents2Metric(ctx, deleteEvents2MetricRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v\n", err)
		return handleRpcErrorWithID(err, "Events2metric", id)
	}
	log.Printf("[INFO] Events2metric %s deleted\n", id)

	d.SetId("")
	return nil
}

func extractCreateE2M(d *schema.ResourceData) *e2m.CreateE2MRequest {
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))
	permutations := expandPermutations(d.Get("permutations"))
	permutationsLimit := wrapperspb.Int32(permutations.GetLimit())
	labels := expandE2MLabels(d.Get("metric_labels"))
	fields := expandE2MFields(d.Get("metric_fields"))
	spansQuery, logsQuery := expandE2MQuery(d)

	e2mParams := &e2m.E2MCreateParams{
		Name:              name,
		Description:       description,
		PermutationsLimit: permutationsLimit,
		MetricLabels:      labels,
		MetricFields:      fields,
	}

	if spansQuery != nil {
		e2mParams.Type = e2m.E2MType_E2M_TYPE_SPANS2METRICS
		e2mParams.Query = spansQuery
	} else if logsQuery != nil {
		e2mParams.Type = e2m.E2MType_E2M_TYPE_LOGS2METRICS
		e2mParams.Query = logsQuery
	}

	return &e2m.CreateE2MRequest{
		E2M: e2mParams,
	}
}

func expandPermutations(v interface{}) *e2m.E2MPermutations {
	l := v.([]interface{})
	if len(l) == 0 || l[0] == nil {
		return &e2m.E2MPermutations{}
	}
	raw := l[0]
	m := raw.(map[string]interface{})
	limit := int32(m["limit"].(int))
	hasExceededLimit := m["has_exceed_limit"].(bool)
	return &e2m.E2MPermutations{
		Limit:            limit,
		HasExceededLimit: hasExceededLimit,
	}
}

func extractUpdateEvents2Metric(d *schema.ResourceData) *e2m.ReplaceE2MRequest {
	id := wrapperspb.String(d.Id())
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))
	permutations := expandPermutations(d.Get("permutations"))
	labels := expandE2MLabels(d.Get("metric_labels"))
	fields := expandE2MFields(d.Get("metric_fields"))
	spansQuery, logsQuery := expandE2MUpdateQuery(d)

	e2mParams := &e2m.E2M{
		Id:           id,
		Name:         name,
		Description:  description,
		Permutations: permutations,
		MetricLabels: labels,
		MetricFields: fields,
	}

	if spansQuery != nil {
		e2mParams.Type = e2m.E2MType_E2M_TYPE_SPANS2METRICS
		e2mParams.Query = spansQuery
	} else if logsQuery != nil {
		e2mParams.Type = e2m.E2MType_E2M_TYPE_LOGS2METRICS
		e2mParams.Query = logsQuery
	}

	return &e2m.ReplaceE2MRequest{
		E2M: e2mParams,
	}
}

func expandE2MLabels(v interface{}) []*e2m.MetricLabel {
	labels := v.(*schema.Set).List()
	result := make([]*e2m.MetricLabel, 0, len(labels))
	for _, l := range labels {
		label := expandE2MLabel(l)
		result = append(result, label)
	}

	return result
}

func expandE2MLabel(v interface{}) *e2m.MetricLabel {
	m := v.(map[string]interface{})
	targetLabel := wrapperspb.String(m["target_label"].(string))
	sourceField := wrapperspb.String(m["source_field"].(string))
	return &e2m.MetricLabel{
		TargetLabel: targetLabel,
		SourceField: sourceField,
	}
}

func expandE2MFields(v interface{}) []*e2m.MetricField {
	v = v.(*schema.Set).List()
	fields := v.([]interface{})
	result := make([]*e2m.MetricField, 0, len(fields))
	for _, f := range fields {
		field := expandE2MField(f)
		result = append(result, field)
	}

	return result
}

func expandE2MField(v interface{}) *e2m.MetricField {
	m := v.(map[string]interface{})
	targetBaseMetricName := wrapperspb.String(m["target_base_metric_name"].(string))
	sourceField := wrapperspb.String(m["source_field"].(string))
	return &e2m.MetricField{
		TargetBaseMetricName: targetBaseMetricName,
		SourceField:          sourceField,
	}
}

func expandE2MQuery(d *schema.ResourceData) (spansQuery *e2m.E2MCreateParams_SpansQuery, logsQuery *e2m.E2MCreateParams_LogsQuery) {
	if spansQueryParams := expandSpansQuery(d.Get("spans_query")); spansQueryParams != nil {
		spansQuery = &e2m.E2MCreateParams_SpansQuery{
			SpansQuery: spansQueryParams,
		}
		return
	}

	if logsQueryParams := expandLogsQuery(d.Get("logs_query")); logsQueryParams != nil {
		logsQuery = &e2m.E2MCreateParams_LogsQuery{
			LogsQuery: logsQueryParams,
		}
	}

	return
}

func expandE2MUpdateQuery(d *schema.ResourceData) (spansQuery *e2m.E2M_SpansQuery, logsQuery *e2m.E2M_LogsQuery) {
	if spansQueryParams := expandSpansQuery(d.Get("spans_query")); spansQueryParams != nil {
		spansQuery = &e2m.E2M_SpansQuery{
			SpansQuery: spansQueryParams,
		}
		return
	}

	if logsQueryParams := expandLogsQuery(d.Get("logs_query")); logsQueryParams != nil {
		logsQuery = &e2m.E2M_LogsQuery{
			LogsQuery: logsQueryParams,
		}
	}

	return
}

func expandSpansQuery(v interface{}) *e2m.SpansQuery {
	l := v.([]interface{})
	if len(l) == 0 || l[0] == nil {
		return nil
	}
	raw := l[0]
	m := raw.(map[string]interface{})

	lucene := wrapperspb.String(m["lucene"].(string))
	applications := interfaceSliceToWrappedStringSlice(m["applications"].(*schema.Set).List())
	subsystems := interfaceSliceToWrappedStringSlice(m["subsystems"].(*schema.Set).List())
	actions := interfaceSliceToWrappedStringSlice(m["actions"].(*schema.Set).List())
	services := interfaceSliceToWrappedStringSlice(m["services"].(*schema.Set).List())

	return &e2m.SpansQuery{
		Lucene:                 lucene,
		ApplicationnameFilters: applications,
		SubsystemnameFilters:   subsystems,
		ActionFilters:          actions,
		ServiceFilters:         services,
	}
}

func expandLogsQuery(v interface{}) *l2m.LogsQuery {
	l := v.([]interface{})
	if len(l) == 0 || l[0] == nil {
		return &l2m.LogsQuery{}
	}
	raw := l[0]
	m := raw.(map[string]interface{})

	searchQuery := wrapperspb.String(m["lucene"].(string))
	applications := interfaceSliceToWrappedStringSlice(m["applications"].(*schema.Set).List())
	subsystems := interfaceSliceToWrappedStringSlice(m["subsystems"].(*schema.Set).List())
	severities := expandSeverities(m["severities"].(*schema.Set).List())

	return &l2m.LogsQuery{
		Lucene:                 searchQuery,
		ApplicationnameFilters: applications,
		SubsystemnameFilters:   subsystems,
		SeverityFilters:        severities,
	}
}

func expandSeverities(severities []interface{}) []l2m.Severity {
	result := make([]l2m.Severity, 0, len(severities))
	for _, s := range severities {
		severity := l2m.Severity(l2m.Severity_value[s.(string)])
		result = append(result, severity)
	}

	return result
}

func Events2MetricSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Type:     schema.TypeString,
			Required: true,
			ValidateFunc: validation.All(
				validation.StringMatch(regexp.MustCompile(`^[A-Za-z\d_:-]*$`), "Invalid metric name, name may only contain ASCII letters and digits, as well as underscores and colons."),
				validation.StringIsNotEmpty,
			),
			Description: "Events2Metric name. Log2Metric names have to be unique per account.",
		},
		"description": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Events2Metric description.",
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
			Computed: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"limit": {
						Type:         schema.TypeInt,
						Optional:     true,
						Computed:     true,
						ValidateFunc: validation.IntAtLeast(0),
						Description:  "defines the permutations' limit of the events2metric.",
					},
					"has_exceed_limit": {
						Type:        schema.TypeBool,
						Computed:    true,
						Description: "notify if the limit permutations' limit of the events2metric has exceed (computed).",
					},
				},
			},
			Description: "defines the permutations' info of the events2metric.",
		},
		"spans_query": {
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
						Description: "An array that contains log’s application names that we want to be alerted on." +
							" Applications can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
						Set: schema.HashString,
					},
					"subsystems": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
						Description: "An array that contains log’s subsystem names that we want to be notified on. " +
							" Subsystems can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
						Set: schema.HashString,
					},
					"actions": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
						Description: "An array that contains log’s actions names that we want to be notified on. " +
							" Actions can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
						Set: schema.HashString,
					},
					"services": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
						Description: "An array that contains log’s services names that we want to be notified on. " +
							" Services can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
						Set: schema.HashString,
					},
				},
			},
			ExactlyOneOf: []string{"spans_query", "logs_query"},
			Description:  "spans-events2metric type. Exactly one of \"spans_query\" or \"logs_query\" should be defined.",
		},
		"logs_query": {
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
						Description: "An array that contains log’s application names that we want to be alerted on." +
							" Applications can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
						Set: schema.HashString,
					},
					"subsystems": {
						Type:     schema.TypeSet,
						Optional: true,
						Elem: &schema.Schema{
							Type: schema.TypeString,
						},
						Description: "An array that contains log’s subsystem names that we want to be notified on. " +
							" Subsystems can be filtered by prefix, suffix, and contains using the next patterns - filter:startsWith:xxx, filter:endsWith:xxx, filter:contains:xxx",
						Set: schema.HashString,
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
			ExactlyOneOf: []string{"spans_query", "logs_query"},
			Description:  "logs-events2metric type. Exactly one of \"spans_query\" or \"logs_query\" should be defined.",
		},
	}
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

func flattenE2MPermutations(permutations *e2m.E2MPermutations) interface{} {
	return []interface{}{map[string]interface{}{
		"limit":            permutations.GetLimit(),
		"has_exceed_limit": permutations.GetHasExceededLimit(),
	},
	}
}

func flattenE2MMetricFields(fields []*e2m.MetricField) interface{} {
	transformed := schema.NewSet(metricFieldsHash(), []interface{}{})
	for _, f := range fields {
		field := flattenE2MMetricField(f)
		transformed.Add(field)
	}
	return transformed
}

func flattenE2MMetricField(field *e2m.MetricField) interface{} {
	return map[string]interface{}{
		"target_base_metric_name": field.GetTargetBaseMetricName().GetValue(),
		"source_field":            field.GetSourceField().GetValue(),
	}
}

func flattenE2MMetricLabels(labels []*e2m.MetricLabel) interface{} {
	result := make([]interface{}, 0, len(labels))
	for _, l := range labels {
		label := flattenE2MMetricLabel(l)
		result = append(result, label)
	}
	return result
}

func flattenE2MMetricLabel(label *e2m.MetricLabel) interface{} {
	return map[string]interface{}{
		"target_label": label.GetTargetLabel().GetValue(),
		"source_field": label.GetSourceField().GetValue(),
	}
}

func flattenSpansQuery(query *e2m.SpansQuery) interface{} {
	m := make(map[string]interface{})

	m["lucene"] = query.GetLucene().GetValue()
	m["applications"] = wrappedStringSliceToStringSlice(query.GetApplicationnameFilters())
	m["subsystems"] = wrappedStringSliceToStringSlice(query.GetSubsystemnameFilters())
	m["actions"] = wrappedStringSliceToStringSlice(query.GetActionFilters())
	m["services"] = wrappedStringSliceToStringSlice(query.GetServiceFilters())

	return []interface{}{m}
}

func flattenLogQuery(query *l2m.LogsQuery) interface{} {
	m := make(map[string]interface{})

	m["lucene"] = query.GetLucene().GetValue()
	m["applications"] = wrappedStringSliceToStringSlice(query.GetApplicationnameFilters())
	m["subsystems"] = wrappedStringSliceToStringSlice(query.GetSubsystemnameFilters())
	m["severities"] = flattenSeverities(query.GetSeverityFilters())

	return []interface{}{m}
}

func flattenSeverities(severities []l2m.Severity) []string {
	result := make([]string, 0, len(severities))
	for _, s := range severities {
		result = append(result, l2m.Severity_name[int32(s)])
	}
	return result
}
