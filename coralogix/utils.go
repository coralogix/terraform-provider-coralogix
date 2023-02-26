package coralogix

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	alertsv1 "terraform-provider-coralogix/coralogix/clientset/grpc/alerts/v1"

	"github.com/hashicorp/go-cty/cty"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	msInHour   = int(time.Hour.Milliseconds())
	msInMinute = int(time.Minute.Milliseconds())
	msInSecond = int(time.Second.Milliseconds())
)

func handleRpcError(err error, resource string) diag.Diagnostics {
	switch status.Code(err) {
	case codes.PermissionDenied, codes.Unauthenticated:
		return diag.Errorf("permission denied for %s endpoint, check your api-key", resource)
	case codes.Internal:
		return diag.Errorf("internal error for %s in Coralogix backend - %s", resource, err)
	case codes.InvalidArgument:
		return diag.Errorf("invalid argument for %s - %s", resource, err)
	default:
		return diag.FromErr(err)
	}
}

func handleRpcErrorWithID(err error, resource, id string) diag.Diagnostics {
	if status.Code(err) == codes.NotFound {
		return diag.Errorf("no %s with id %s found", resource, id)
	}
	return handleRpcError(err, resource)
}

// datasourceSchemaFromResourceSchema is a recursive func that
// converts an existing Resource schema to a Datasource schema.
// All schema elements are copied, but certain attributes are ignored or changed:
// - all attributes have Computed = true
// - all attributes have ForceNew, Required = false
// - Validation funcs and attributes (e.g. MaxItems) are not copied
func datasourceSchemaFromResourceSchema(rs map[string]*schema.Schema) map[string]*schema.Schema {
	ds := make(map[string]*schema.Schema, len(rs))
	for k, v := range rs {
		dv := &schema.Schema{
			Computed:    true,
			ForceNew:    false,
			Required:    false,
			Description: v.Description,
			Type:        v.Type,
		}

		switch v.Type {
		case schema.TypeSet:
			dv.Set = v.Set
			fallthrough
		case schema.TypeList:
			// List & Set types are generally used for 2 cases:
			// - a list/set of simple primitive values (e.g. list of strings)
			// - a sub resource
			if elem, ok := v.Elem.(*schema.Resource); ok {
				// handle the case where the Element is a sub-resource
				dv.Elem = &schema.Resource{
					Schema: datasourceSchemaFromResourceSchema(elem.Schema),
				}
			} else {
				// handle simple primitive case
				dv.Elem = v.Elem
			}

		default:
			// Elem of all other types are copied as-is
			dv.Elem = v.Elem

		}
		ds[k] = dv

	}
	return ds
}

func interfaceSliceToStringSlice(s []interface{}) []string {
	result := make([]string, 0, len(s))
	for _, v := range s {
		result = append(result, v.(string))
	}
	return result
}

func interfaceSliceToWrappedStringSlice(s []interface{}) []*wrapperspb.StringValue {
	result := make([]*wrapperspb.StringValue, 0, len(s))
	for _, v := range s {
		result = append(result, wrapperspb.String(v.(string)))
	}
	return result
}

func wrappedStringSliceToStringSlice(s []*wrapperspb.StringValue) []string {
	result := make([]string, 0, len(s))
	for _, v := range s {
		result = append(result, v.GetValue())
	}
	return result
}

func timeInDaySchema(description string) *schema.Schema {
	timeRegex := regexp.MustCompile(`^(0\d|1\d|2[0-3]):[0-5]\d$`)
	return &schema.Schema{
		Type:         schema.TypeString,
		Required:     true,
		ValidateFunc: validation.StringMatch(timeRegex, "not valid time, only HH:MM format is allowed"),
		Description:  description,
	}
}

func expandTimeInDay(v interface{}) *alertsv1.Time {
	timeArr := strings.Split(v.(string), ":")
	hours, _ := strconv.Atoi(timeArr[0])
	minutes, _ := strconv.Atoi(timeArr[1])
	return &alertsv1.Time{
		Hours:   int32(hours),
		Minutes: int32(minutes),
	}
}

func flattenTimeInDay(t *alertsv1.Time, utc int32) string {
	hours := convertGmtToUtc(t.GetHours(), utc)
	hoursStr := toTwoDigitsFormat(hours)
	minStr := toTwoDigitsFormat(t.GetMinutes())
	return fmt.Sprintf("%s:%s", hoursStr, minStr)
}

func toTwoDigitsFormat(digit int32) string {
	digitStr := fmt.Sprintf("%d", digit)
	if len(digitStr) == 1 {
		digitStr = "0" + digitStr
	}
	return digitStr
}

func timeSchema(description string) *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Optional: true,
		MaxItems: 1,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"hours": {
					Type:         schema.TypeInt,
					Optional:     true,
					ValidateFunc: validation.IntAtLeast(0),
				},
				"minutes": {
					Type:         schema.TypeInt,
					Optional:     true,
					ValidateFunc: validation.IntAtLeast(0),
				},
				"seconds": {
					Type:         schema.TypeInt,
					Optional:     true,
					ValidateFunc: validation.IntAtLeast(0),
				},
			},
		},
		Description: description,
	}
}

func expandTimeToMS(v interface{}) int {
	l := v.([]interface{})
	if len(l) == 0 {
		return 0
	}

	m := l[0].(map[string]interface{})

	timeMS := msInHour * m["hours"].(int)
	timeMS += msInMinute * m["minutes"].(int)
	timeMS += msInSecond * m["seconds"].(int)

	return timeMS
}

func flattenTimeframe(timeMS int) []interface{} {
	if timeMS == 0 {
		return nil
	}

	hours := timeMS / msInHour
	timeMS -= hours * msInHour

	minutes := timeMS / msInMinute
	timeMS -= minutes * msInMinute

	seconds := timeMS / msInSecond

	return []interface{}{map[string]int{
		"hours":   hours,
		"minutes": minutes,
		"seconds": seconds,
	}}
}

func sliceToString(data []string) string {
	b, _ := json.Marshal(data)
	return fmt.Sprintf("%v", string(b))
}

func randFloat() float64 {
	r := rand.New(rand.NewSource(99))
	return r.Float64()
}

func selectRandomlyFromSlice(s []string) string {
	return s[acctest.RandIntRange(0, len(s))]
}

func selectManyRandomlyFromSlice(s []string) []string {
	r := rand.New(rand.NewSource(99))
	indexPerms := r.Perm(len(s))
	itemsToSelect := acctest.RandIntRange(0, len(s)+1)
	result := make([]string, 0, itemsToSelect)
	for _, index := range indexPerms {
		result = append(result, s[index])
	}
	return result
}

func getKeysStrings(m map[string]string) []string {
	result := make([]string, 0)
	for k := range m {
		result = append(result, k)
	}
	return result
}

func getKeysInterface(m map[string]interface{}) []string {
	result := make([]string, 0)
	for k := range m {
		result = append(result, k)
	}
	return result
}

func getKeysInt32(m map[string]int32) []string {
	result := make([]string, 0)
	for k := range m {
		result = append(result, k)
	}
	return result
}

func getKeysInt(m map[string]int) []string {
	result := make([]string, 0)
	for k := range m {
		result = append(result, k)
	}
	return result
}

func getKeysRelativeTimeFrame(m map[string]protoTimeFrameAndRelativeTimeFrame) []string {
	result := make([]string, 0)
	for k := range m {
		result = append(result, k)
	}
	return result
}

func reverseMapStrings(m map[string]string) map[string]string {
	n := make(map[string]string)
	for k, v := range m {
		n[v] = k
	}
	return n
}

func reverseMapIntToString(m map[string]int) map[int]string {
	n := make(map[int]string)
	for k, v := range m {
		n[v] = k
	}
	return n
}

func reverseMapRelativeTimeFrame(m map[string]protoTimeFrameAndRelativeTimeFrame) map[protoTimeFrameAndRelativeTimeFrame]string {
	n := make(map[protoTimeFrameAndRelativeTimeFrame]string)
	for k, v := range m {
		n[v] = k
	}
	return n
}

func strToUint32(str string) uint32 {
	n, _ := strconv.ParseUint(str, 10, 32)
	return uint32(n)
}

func uint32ToStr(n uint32) string {
	return strconv.FormatUint(uint64(n), 10)
}

func mailValidationFunc() schema.SchemaValidateDiagFunc {
	return func(v interface{}, _ cty.Path) diag.Diagnostics {
		mailStr := v.(string)
		matched, _ := regexp.MatchString(`^[a-z/d._%+\-]+@[a-z/d.\-]+\.[a-z]{2,4}$`, mailStr)
		if !matched {
			return diag.Errorf("%s is not a valid mail address", mailStr)
		}
		return nil
	}
}

func utcValidationFunc() schema.SchemaValidateDiagFunc {
	return func(v interface{}, _ cty.Path) diag.Diagnostics {
		mailStr := v.(string)
		matched, _ := regexp.MatchString(`/^UTC[+-]\d{2}:\d{2}$/g`, mailStr)
		if !matched {
			return diag.Errorf("%s is not a valid mail address", mailStr)
		}
		return nil
	}
}

func urlValidationFunc() schema.SchemaValidateDiagFunc {
	return func(v interface{}, _ cty.Path) diag.Diagnostics {
		if _, err := url.ParseRequestURI(v.(string)); err != nil {
			return diag.Errorf("%s in not valid url - %s", v.(string), err.Error())
		}
		return nil
	}
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func SuppressEquivalentJSONDiffs(k, old, new string, d *schema.ResourceData) bool {
	return JSONStringsEqual(old, new)
}

func JSONStringsEqual(s1, s2 string) bool {
	b1 := bytes.NewBufferString("")
	if err := json.Compact(b1, []byte(s1)); err != nil {
		return false
	}

	b2 := bytes.NewBufferString("")
	if err := json.Compact(b2, []byte(s2)); err != nil {
		return false
	}

	return JSONBytesEqual(b1.Bytes(), b2.Bytes())
}

func JSONBytesEqual(b1, b2 []byte) bool {
	var o1 interface{}
	if err := json.Unmarshal(b1, &o1); err != nil {
		return false
	}

	var o2 interface{}
	if err := json.Unmarshal(b2, &o2); err != nil {
		return false
	}

	return reflect.DeepEqual(o1, o2)
}
