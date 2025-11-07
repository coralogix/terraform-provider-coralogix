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

package enrichment_rules

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"google.golang.org/protobuf/encoding/protojson"

	"google.golang.org/grpc/codes"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var fileContentLimit = int(1e6)

func ResourceCoralogixDataSet() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCoralogixDataSetCreate,
		ReadContext:   resourceCoralogixDataSetRead,
		UpdateContext: resourceCoralogixDataSetUpdate,
		DeleteContext: resourceCoralogixDataSetDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(120 * time.Second),
			Read:   schema.DefaultTimeout(60 * time.Second),
			Update: schema.DefaultTimeout(120 * time.Second),
			Delete: schema.DefaultTimeout(30 * time.Second),
		},
		Description:        "**Note:** Data Sets will be removed in a future version of the Terraform Provider. Please use the API directly for creating custom enrichments: https://github.com/coralogix/coralogix-management-sdk/",
		Schema:             DataSetSchema(),
		DeprecationMessage: "Data Sets will be removed in a future version of the Terraform Provider. Please use the API directly for creating custom enrichments: https://github.com/coralogix/coralogix-management-sdk/",
	}
}

func DataSetSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringIsNotEmpty,
		},
		"description": {
			Type:     schema.TypeString,
			Optional: true,
		},
		"version": {
			Type:     schema.TypeInt,
			Computed: true,
		},
		"file_content": {
			Type:         schema.TypeString,
			Optional:     true,
			ExactlyOneOf: []string{"file_content", "uploaded_file"},
			ValidateFunc: fileContentNoLongerThan,
		},
		"uploaded_file": {
			Type:     schema.TypeList,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"path": {
						Type:     schema.TypeString,
						Required: true,
						//ValidateFunc: validation.StringMatch(
						//regexp.MustCompile(`^(?:\w\:|\/)(\/[a-z_\-\s\d\.]+)+\.csv$`), "not valid path or not csv file"),
					},
					"modification_time_uploaded": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"updated_from_uploading": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  false,
					},
				},
			},
			Optional:     true,
			ExactlyOneOf: []string{"file_content", "uploaded_file"},
		},
	}
}

func fileContentNoLongerThan(i interface{}, k string) ([]string, []error) {
	v, ok := i.(string)
	if !ok {
		return nil, []error{fmt.Errorf("expected type of %q to be string", k)}
	}

	fileLength := len(v)
	if fileLength > fileContentLimit {
		return nil, []error{fmt.Errorf("file_content expected to be no longer than %d charicters, got %d charicters", fileContentLimit, fileLength)}
	}

	return nil, nil
}

func resourceCoralogixDataSetCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	req, fileModificationTime, err := expandDataSetRequest(d)
	if err != nil {
		log.Printf("[ERROR] Received error while expanding enrichment-data: %s", err.Error())
		return diag.FromErr(err)
	}
	log.Printf("[INFO] Creating new enrichment-data: %s", protojson.Format(req))

	resp, err := meta.(*clientset.ClientSet).DataSet().Create(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf("%s", utils.FormatRpcErrors(err, cxsdk.CreateDataSetRPC, protojson.Format(req)))
	}

	if uploadedFile, ok := d.GetOk("uploaded_file"); ok {
		if err := setModificationTimeUploaded(d, uploadedFile, fileModificationTime); err != nil {
			return diag.FromErr(err)
		}
	}

	id := utils.Uint32ToStr(resp.GetCustomEnrichment().GetId())
	d.SetId(id)

	return resourceCoralogixDataSetRead(ctx, d, meta)
}

func setModificationTimeUploaded(d *schema.ResourceData, uploadedFile interface{}, modificationTime string) error {
	uploadedFileMap := uploadedFile.([]interface{})[0].(map[string]interface{})
	uploadedFileMap["updated_from_uploading"] = false
	uploadedFileMap["modification_time_uploaded"] = modificationTime
	return d.Set("uploaded_file", []interface{}{uploadedFileMap})
}

func resourceCoralogixDataSetRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	req := &cxsdk.GetDataSetRequest{Id: wrapperspb.UInt32(utils.StrToUint32(id))}

	log.Print("[INFO] Reading enrichment-data")
	DataSetResp, err := meta.(*clientset.ClientSet).DataSet().Get(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			d.SetId("")
			return diag.Diagnostics{diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  fmt.Sprintf("DataSet %q is in state, but no longer exists in Coralogix backend", id),
				Detail:   fmt.Sprintf("%s will be recreated when you apply", id),
			}}
		}
		return diag.Errorf("%s", utils.FormatRpcErrors(err, cxsdk.GetDataSetRPC, protojson.Format(req)))
	}

	log.Printf("[INFO] Received enrichment-data: %s", protojson.Format(DataSetResp))
	return setDataSet(d, DataSetResp.GetCustomEnrichment())
}

func resourceCoralogixDataSetUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	req, fileModificationTime, err := expandUpdateDataSetRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Print("[INFO] Updating enrichment-data")
	_, err = meta.(*clientset.ClientSet).DataSet().Update(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf("%s", utils.FormatRpcErrors(err, cxsdk.UpdateDataSetRPC, protojson.Format(req)))
	}

	if uploadedFile, ok := d.GetOk("uploaded_file"); ok {
		if err = setModificationTimeUploaded(d, uploadedFile, fileModificationTime); err != nil {
			return diag.FromErr(err)
		}
	}

	return resourceCoralogixDataSetRead(ctx, d, meta)
}

func resourceCoralogixDataSetDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	req := &cxsdk.DeleteDataSetRequest{CustomEnrichmentId: wrapperspb.UInt32(utils.StrToUint32(id))}

	log.Printf("[INFO] Deleting enrichment-data %s", id)
	_, err := meta.(*clientset.ClientSet).DataSet().Delete(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf("%s", utils.FormatRpcErrors(err, cxsdk.DeleteDataSetRPC, protojson.Format(req)))
	}

	log.Printf("[INFO] enrichment-data %s deleted", id)

	d.SetId("")
	return nil
}

func setDataSet(d *schema.ResourceData, c *cxsdk.DataSet) diag.Diagnostics {
	if err := d.Set("name", c.Name); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("description", c.Description); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("version", int(c.Version)); err != nil {
		return diag.FromErr(err)
	}

	uploadedFile, err := flattenUploadedFile(d)
	if err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("uploaded_file", uploadedFile); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func expandDataSetRequest(d *schema.ResourceData) (*cxsdk.CreateDataSetRequest, string, error) {
	name, description, file, modificationTime, err := expandEnrichmentReq(d)
	if err != nil {
		return nil, "", err
	}
	req := &cxsdk.CreateDataSetRequest{
		Name:        name,
		Description: description,
		File:        file,
	}
	return req, modificationTime, nil
}

func expandUpdateDataSetRequest(d *schema.ResourceData) (*cxsdk.UpdateDataSetRequest, string, error) {
	customEnrichmentId := wrapperspb.UInt32(utils.StrToUint32(d.Id()))
	name, description, file, modificationTime, err := expandEnrichmentReq(d)
	if err != nil {
		return nil, "", err
	}
	req := &cxsdk.UpdateDataSetRequest{
		CustomEnrichmentId: customEnrichmentId,
		Name:               name,
		Description:        description,
		File:               file,
	}
	return req, modificationTime, nil
}

func expandEnrichmentReq(d *schema.ResourceData) (*wrapperspb.StringValue, *wrapperspb.StringValue, *cxsdk.File, string, error) {
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))
	file, modificationTime, err := expandFileAndModificationTime(d)
	return name, description, file, modificationTime, err
}

func expandFileAndModificationTime(d *schema.ResourceData) (*cxsdk.File, string, error) {
	fileContent, modificationTime, err := expandFileContent(d)
	if err != nil {
		return nil, modificationTime, err
	}

	return &cxsdk.File{
		Name:      wrapperspb.String(" "),
		Extension: wrapperspb.String("csv"),
		Content:   &cxsdk.FileTextual{Textual: wrapperspb.String(fileContent)},
	}, modificationTime, nil
}

func expandFileContent(d *schema.ResourceData) (fileContent string, modificationTime string, err error) {
	if fileContent, ok := d.GetOk("file_content"); !ok {
		uploadedFile := d.Get("uploaded_file").([]interface{})[0].(map[string]interface{})

		path := uploadedFile["path"].(string)

		f, err := os.Open(path)
		if err != nil {
			return "", "", err
		}

		csvReader := csv.NewReader(f)
		for {
			rec, err := csvReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", "", err
			}
			fileContent = strings.Join(rec, "")
		}

		stats, err := os.Stat(path)
		if err != nil {
			return "", "", err
		}
		modificationTime := stats.ModTime().String()

		return fileContent.(string), modificationTime, nil
	} else {
		return fileContent.(string), "", nil
	}
}

func flattenUploadedFile(d *schema.ResourceData) (interface{}, error) {
	if uploadedFile, ok := d.GetOk("uploaded_file"); ok {
		uploadedFileMap := uploadedFile.([]interface{})[0].(map[string]interface{})
		path := uploadedFileMap["path"].(string)
		stat, err := os.Stat(path)
		if err != nil {
			return nil, err
		}

		if stat.ModTime().String() != uploadedFileMap["modification_time_uploaded"] {
			uploadedFileMap["updated_from_uploading"] = true
		}

		return []interface{}{uploadedFileMap}, nil
	}

	return nil, nil
}
