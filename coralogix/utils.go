// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package coralogix

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math/big"
	"math/rand"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	gouuid "github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	diag2 "github.com/hashicorp/terraform-plugin-framework/diag"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func formatRpcErrors(err error, url, requestStr string) string {
	switch status.Code(err) {
	case codes.Internal:
		return fmt.Sprintf("internal error in Coralogix backend.\nerror - %s\nurl - %s\nrequest - %s", err, url, requestStr)
	case codes.InvalidArgument:
		return fmt.Sprintf("invalid argument error.\nerror - %s\nurl - %s\nrequest - %s", err, url, requestStr)
	default:
		return err.Error()
	}
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

func frameworkDatasourceSchemaFromFrameworkResourceSchema(rs resourceschema.Schema) datasourceschema.Schema {
	attributes := convertAttributes(rs.Attributes)
	if idSchema, ok := rs.Attributes["id"]; ok {
		attributes["id"] = datasourceschema.StringAttribute{
			Required:            true,
			Description:         idSchema.GetDescription(),
			MarkdownDescription: idSchema.GetMarkdownDescription(),
		}
	}

	return datasourceschema.Schema{
		Attributes: attributes,
		//Blocks: convertBlocks(rs.Blocks),
		Description:         rs.Description,
		MarkdownDescription: rs.MarkdownDescription,
		DeprecationMessage:  rs.DeprecationMessage,
	}
}

func convertAttributes(attributes map[string]resourceschema.Attribute) map[string]datasourceschema.Attribute {
	result := make(map[string]datasourceschema.Attribute, len(attributes))
	for k, v := range attributes {
		result[k] = convertAttribute(v)
	}
	return result
}

func convertAttribute(resourceAttribute resourceschema.Attribute) datasourceschema.Attribute {
	switch attr := resourceAttribute.(type) {
	case resourceschema.BoolAttribute:
		return datasourceschema.BoolAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}
	case resourceschema.Float64Attribute:
		return datasourceschema.Float64Attribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}
	case resourceschema.Int64Attribute:
		return datasourceschema.Int64Attribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}
	case resourceschema.NumberAttribute:
		return datasourceschema.NumberAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}
	case resourceschema.StringAttribute:
		return datasourceschema.StringAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}
	case resourceschema.MapAttribute:
		return datasourceschema.MapAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			ElementType:         attr.ElementType,
		}
	case resourceschema.ObjectAttribute:
		return datasourceschema.ObjectAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			AttributeTypes:      attr.AttributeTypes,
		}
	case resourceschema.SetAttribute:
		return datasourceschema.SetAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			ElementType:         attr.ElementType,
		}
	case resourceschema.ListNestedAttribute:
		return datasourceschema.ListNestedAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			NestedObject: datasourceschema.NestedAttributeObject{
				Attributes: convertAttributes(attr.NestedObject.Attributes),
			},
		}
	case resourceschema.ListAttribute:
		return datasourceschema.ListAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			ElementType:         attr.ElementType,
		}
	case resourceschema.MapNestedAttribute:
		return datasourceschema.MapNestedAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			NestedObject: datasourceschema.NestedAttributeObject{
				Attributes: convertAttributes(attr.NestedObject.Attributes),
			},
		}
	case resourceschema.SetNestedAttribute:
		return datasourceschema.SetNestedAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			NestedObject: datasourceschema.NestedAttributeObject{
				Attributes: convertAttributes(attr.NestedObject.Attributes),
			},
		}
	case resourceschema.SingleNestedAttribute:
		return datasourceschema.SingleNestedAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
			Attributes:          convertAttributes(attr.Attributes),
		}
	case resourceschema.DynamicAttribute:
		return datasourceschema.DynamicAttribute{
			Computed:            true,
			Description:         attr.Description,
			MarkdownDescription: attr.MarkdownDescription,
		}
	default:
		panic(fmt.Sprintf("unknown resource attribute type: %T", resourceAttribute))
	}
}

func interfaceSliceToStringSlice(s []interface{}) []string {
	result := make([]string, 0, len(s))
	for _, v := range s {
		result = append(result, v.(string))
	}
	return result
}

func attrSliceToFloat32Slice(ctx context.Context, arr []attr.Value) ([]float32, diag2.Diagnostics) {
	var diags diag2.Diagnostics
	result := make([]float32, 0, len(arr))
	for _, v := range arr {
		val, err := v.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}
		var d big.Float
		if err = val.As(&d); err != nil {
			diags.AddError("Failed to convert value to float64", err.Error())
			continue
		}
		f, _ := d.Float64()
		result = append(result, float32(f))
	}
	return result, diags
}

func float32SliceTypeList(ctx context.Context, arr []float32) (types.List, diag2.Diagnostics) {
	if len(arr) == 0 {
		return types.ListNull(types.Float64Type), nil
	}
	result := make([]attr.Value, 0, len(arr))
	for _, v := range arr {
		if float32(int(v)) != v {
			result = append(result, types.Float64Value(float64(v*10000)/float64(10000)))
		} else {
			result = append(result, types.Float64Value(float64(v)))
		}
	}
	return types.ListValueFrom(ctx, types.Float64Type, result)
}

func wrappedStringSliceToTypeStringSet(s []*wrapperspb.StringValue) types.Set {
	if len(s) == 0 {
		return types.SetNull(types.StringType)
	}
	elements := make([]attr.Value, 0, len(s))
	for _, v := range s {
		elements = append(elements, types.StringValue(v.GetValue()))
	}
	return types.SetValueMust(types.StringType, elements)
}

func stringSliceToTypeStringSet(s []string) types.Set {
	if len(s) == 0 {
		return types.SetNull(types.StringType)
	}
	elements := make([]attr.Value, 0, len(s))
	for _, v := range s {
		elements = append(elements, types.StringValue(v))
	}
	return types.SetValueMust(types.StringType, elements)
}

func int32SliceToTypeInt64Set(arr []int32) types.Set {
	if len(arr) == 0 {
		return types.SetNull(types.Int64Type)
	}
	elements := make([]attr.Value, 0, len(arr))
	for _, n := range arr {
		elements = append(elements, types.Int64Value(int64(n)))
	}
	return types.SetValueMust(types.StringType, elements)
}

func wrappedStringSliceToTypeStringList(s []*wrapperspb.StringValue) types.List {
	if len(s) == 0 {
		return types.ListNull(types.StringType)
	}
	elements := make([]attr.Value, 0, len(s))
	for _, v := range s {
		elements = append(elements, types.StringValue(v.GetValue()))
	}
	return types.ListValueMust(types.StringType, elements)
}

func typeStringSliceToWrappedStringSlice(ctx context.Context, s []attr.Value) ([]*wrapperspb.StringValue, diag2.Diagnostics) {
	var diags diag2.Diagnostics
	result := make([]*wrapperspb.StringValue, 0, len(s))
	for _, v := range s {
		val, err := v.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}
		var str string

		if err = val.As(&str); err != nil {
			diags.AddError("Failed to convert value to string", err.Error())
			continue
		}
		result = append(result, wrapperspb.String(str))
	}
	return result, diags
}

func typeInt64ToWrappedInt64(v types.Int64) *wrapperspb.Int64Value {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return wrapperspb.Int64(v.ValueInt64())
}

func typeInt64ToWrappedInt32(v types.Int64) *wrapperspb.Int32Value {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return wrapperspb.Int32(int32(v.ValueInt64()))
}

func typeInt64ToWrappedUint32(v types.Int64) *wrapperspb.UInt32Value {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return wrapperspb.UInt32(uint32(v.ValueInt64()))
}

func typeNumberToWrappedUint64(v types.Number) *wrapperspb.UInt64Value {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	number, _ := v.ValueBigFloat().Uint64()
	return wrapperspb.UInt64(number)
}

func wrappedUint64TotypeNumber(v *wrapperspb.UInt64Value) types.Number {
	if v == nil {
		return types.NumberNull()
	}
	return types.NumberValue(big.NewFloat(float64(v.GetValue())))
}

func typeBoolToWrapperspbBool(v types.Bool) *wrapperspb.BoolValue {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return wrapperspb.Bool(v.ValueBool())
}

func typeStringSliceToStringSlice(ctx context.Context, s []attr.Value) ([]string, diag2.Diagnostics) {
	result := make([]string, 0, len(s))
	var diags diag2.Diagnostics
	for _, v := range s {
		val, err := v.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}
		var str string
		if err = val.As(&str); err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}
		result = append(result, str)
	}
	if diags.HasError() {
		return nil, diags
	}
	return result, nil
}

func typeInt64SliceToInt32Slice(ctx context.Context, s []attr.Value) ([]int32, diag2.Diagnostics) {
	result := make([]int32, 0, len(s))
	var diags diag2.Diagnostics
	for _, v := range s {
		val, err := v.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}
		var n int64
		if err = val.As(&n); err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}
		result = append(result, int32(n))
	}
	if diags.HasError() {
		return nil, diags
	}
	return result, nil
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

func objIsNullOrUnknown(obj types.Object) bool {
	return obj.IsNull() || obj.IsUnknown()
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

func strToUint32(str string) uint32 {
	n, _ := strconv.ParseUint(str, 10, 32)
	return uint32(n)
}

func uint32ToStr(n uint32) string {
	return strconv.FormatUint(uint64(n), 10)
}

type urlValidationFuncFramework struct {
}

func (u urlValidationFuncFramework) Description(_ context.Context) string {
	return "string must be a valid url format"
}

func (u urlValidationFuncFramework) MarkdownDescription(ctx context.Context) string {
	return u.Description(ctx)
}

func (u urlValidationFuncFramework) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()

	if _, err := url.ParseRequestURI(value); err != nil {
		resp.Diagnostics.Append(
			diag2.NewAttributeErrorDiagnostic(
				req.Path,
				"Invalid Attribute Value Format",
				fmt.Sprintf("Attribute %s in not a valid url - %s", req.Path, value),
			),
		)
	}
}

func flattenUtc(timeZone string) int32 {
	utcStr := strings.Split(timeZone, "UTC")[1]
	utc, _ := strconv.Atoi(utcStr)
	return int32(utc)
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
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

func randBool() bool {
	return rand.Int()%2 == 0
}

func typeStringToWrapperspbString(str types.String) *wrapperspb.StringValue {
	if str.IsNull() || str.IsUnknown() {
		return nil

	}
	return wrapperspb.String(str.ValueString())
}

func typeStringToStringPointer(str types.String) *string {
	if str.IsNull() || str.IsUnknown() {
		return nil
	}
	result := new(string)
	*result = str.ValueString()
	return result
}

func stringPointerToTypeString(str *string) types.String {
	if str == nil {
		return types.StringNull()
	}
	return types.StringValue(*str)
}

func typeFloat64ToWrapperspbDouble(num types.Float64) *wrapperspb.DoubleValue {
	if num.IsNull() {
		return nil
	}

	return wrapperspb.Double(num.ValueFloat64())
}

func wrapperspbStringToTypeString(str *wrapperspb.StringValue) types.String {
	if str == nil {
		return types.StringNull()
	}

	return types.StringValue(str.GetValue())
}

func wrapperspbInt64ToTypeInt64(num *wrapperspb.Int64Value) types.Int64 {
	if num == nil {
		return types.Int64Null()
	}

	return types.Int64Value(num.GetValue())
}

func wrapperspbUInt64ToNumberType(num *wrapperspb.UInt64Value) types.Number {
	if num == nil {
		return types.NumberNull()
	}

	return types.NumberValue(big.NewFloat(float64(num.GetValue())))
}

func numberTypeToWrapperspbUInt64(num types.Number) *wrapperspb.UInt64Value {
	if num.IsNull() {
		return nil
	}
	// types.NumberValue(big.NewFloat(float64(num.GetValue())))
	val, _ := num.ValueBigFloat().Uint64()
	return wrapperspb.UInt64(val)
}

func wrapperspbUint32ToTypeInt64(num *wrapperspb.UInt32Value) types.Int64 {
	if num == nil {
		return types.Int64Null()
	}

	return types.Int64Value(int64(num.GetValue()))
}

func wrapperspbDoubleToTypeFloat64(num *wrapperspb.DoubleValue) types.Float64 {
	if num == nil {
		return types.Float64Null()
	}

	return types.Float64Value(num.GetValue())
}

func wrapperspbBoolToTypeBool(b *wrapperspb.BoolValue) types.Bool {
	if b == nil {
		return types.BoolNull()
	}

	return types.BoolValue(b.GetValue())
}

func wrapperspbInt32ToTypeInt64(num *wrapperspb.Int32Value) types.Int64 {
	if num == nil {
		return types.Int64Null()
	}

	return types.Int64Value(int64(num.GetValue()))
}

func ReverseMap[K, V comparable](m map[K]V) map[V]K {
	n := make(map[V]K)
	for k, v := range m {
		n[v] = k
	}
	return n
}

func GetKeys[K cmp.Ordered, V comparable](m map[K]V) []K {
	return slices.Sorted(maps.Keys(m))
}

func GetValues[K, V cmp.Ordered](m map[K]V) []V {
	return slices.Sorted(maps.Values(m))
}

func parseNumInt32(desired string) int32 {
	parsed, err := strconv.ParseInt(desired, 10, 32)
	if err != nil {
		return 0
	}
	return int32(parsed)
}

func parseNumUint32(desired string) uint32 {
	parsed, err := strconv.ParseUint(desired, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(parsed)
}

func typeMapToStringMap(ctx context.Context, m types.Map) (map[string]string, diag2.Diagnostics) {
	var result map[string]string
	diags := m.ElementsAs(ctx, &result, true)
	return result, diags
}

func expandUuid(uuid types.String) *wrapperspb.StringValue {
	if uuid.IsNull() || uuid.IsUnknown() {
		return &wrapperspb.StringValue{Value: gouuid.NewString()}
	}
	return &wrapperspb.StringValue{Value: uuid.ValueString()}
}

func retryableStatusCode(statusCode codes.Code) bool {
	switch statusCode {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Aborted:
		return true
	default:
		return false
	}
}

func uint32SliceToWrappedUint32Slice(s []uint32) []*wrapperspb.UInt32Value {
	result := make([]*wrapperspb.UInt32Value, 0, len(s))
	for _, n := range s {
		result = append(result, wrapperspb.UInt32(n))
	}
	return result
}

func convertSchemaWithoutID(rs resourceschema.Schema) datasourceschema.Schema {
	attributes := convertAttributes(rs.Attributes)
	return datasourceschema.Schema{
		Attributes:          attributes,
		Description:         rs.Description,
		MarkdownDescription: rs.MarkdownDescription,
		DeprecationMessage:  rs.DeprecationMessage,
	}
}

func typeStringToWrapperspbUint32(str types.String) (*wrapperspb.UInt32Value, diag2.Diagnostics) {
	parsed, err := strconv.ParseUint(str.ValueString(), 10, 32)
	if err != nil {
		return nil, diag2.Diagnostics{diag2.NewErrorDiagnostic("Failed to convert string to uint32", err.Error())}
	}
	return wrapperspb.UInt32(uint32(parsed)), nil
}

func WrapperspbUint32ToString(num *wrapperspb.UInt32Value) types.String {
	if num == nil {
		return types.StringNull()
	}
	return types.StringValue(strconv.FormatUint(uint64(num.GetValue()), 10))

}
