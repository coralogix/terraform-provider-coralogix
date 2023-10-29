package coralogix

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	enrichment "github.com/coralogix/coralogix-sdk-demo/enrichment/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var fileContentLimit = int(1e6)

func resourceCoralogixDataSet() *schema.Resource {
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

		Schema: DataSetSchema(),
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
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "enrichment-data")
	}
	log.Printf("[INFO] Creating new enrichment-data: %#v", req)

	resp, err := meta.(*clientset.ClientSet).DataSet().CreatDataSet(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "enrichment-data")
	}

	if uploadedFile, ok := d.GetOk("uploaded_file"); ok {
		if err := setModificationTimeUploaded(d, uploadedFile, fileModificationTime); err != nil {
			return diag.FromErr(err)
		}
	}

	id := uint32ToStr(resp.GetCustomEnrichment().GetId())
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
	req := &enrichment.GetCustomEnrichmentRequest{Id: wrapperspb.UInt32(strToUint32(id))}

	log.Print("[INFO] Reading enrichment-data")
	DataSetResp, err := meta.(*clientset.ClientSet).DataSet().GetDataSet(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			d.SetId("")
			return diag.Diagnostics{diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  fmt.Sprintf("DataSet %q is in state, but no longer exists in Coralogix backend", id),
				Detail:   fmt.Sprintf("%s will be recreated when you apply", id),
			}}
		}
		return handleRpcErrorWithID(err, "enrichment-data", id)
	}

	log.Printf("[INFO] Received enrichment-data: %#v", DataSetResp)
	return setDataSet(d, DataSetResp.GetCustomEnrichment())
}

func resourceCoralogixDataSetUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	req, fileModificationTime, err := expandUpdateDataSetRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Print("[INFO] Updating enrichment-data")
	_, err = meta.(*clientset.ClientSet).DataSet().UpdateDataSet(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "enrichment-data")
	}

	if uploadedFile, ok := d.GetOk("uploaded_file"); ok {
		if err := setModificationTimeUploaded(d, uploadedFile, fileModificationTime); err != nil {
			return diag.FromErr(err)
		}
	}

	return resourceCoralogixDataSetRead(ctx, d, meta)
}

func resourceCoralogixDataSetDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	req := &enrichment.DeleteCustomEnrichmentRequest{CustomEnrichmentId: wrapperspb.UInt32(strToUint32(id))}

	log.Printf("[INFO] Deleting enrichment-data %s\n", id)
	_, err := meta.(*clientset.ClientSet).DataSet().DeleteDataSet(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v\n", err)
		return handleRpcErrorWithID(err, "enrichment-data", id)
	}

	log.Printf("[INFO] enrichment-data %s deleted\n", id)

	d.SetId("")
	return nil
}

func setDataSet(d *schema.ResourceData, c *enrichment.CustomEnrichment) diag.Diagnostics {
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

func expandDataSetRequest(d *schema.ResourceData) (*enrichment.CreateCustomEnrichmentRequest, string, error) {
	name, description, file, modificationTime, err := expandEnrichmentReq(d)
	if err != nil {
		return nil, "", err
	}
	req := &enrichment.CreateCustomEnrichmentRequest{
		Name:        name,
		Description: description,
		File:        file,
	}
	return req, modificationTime, nil
}

func expandUpdateDataSetRequest(d *schema.ResourceData) (*enrichment.UpdateCustomEnrichmentRequest, string, error) {
	customEnrichmentId := wrapperspb.UInt32(strToUint32(d.Id()))
	name, description, file, modificationTime, err := expandEnrichmentReq(d)
	if err != nil {
		return nil, "", err
	}
	req := &enrichment.UpdateCustomEnrichmentRequest{
		CustomEnrichmentId: customEnrichmentId,
		Name:               name,
		Description:        description,
		File:               file,
	}
	return req, modificationTime, nil
}

func expandEnrichmentReq(d *schema.ResourceData) (*wrapperspb.StringValue, *wrapperspb.StringValue, *enrichment.File, string, error) {
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))
	file, modificationTime, err := expandFileAndModificationTime(d)
	return name, description, file, modificationTime, err
}

func expandFileAndModificationTime(d *schema.ResourceData) (*enrichment.File, string, error) {
	fileContent, modificationTime, err := expandFileContent(d)
	if err != nil {
		return nil, modificationTime, err
	}

	return &enrichment.File{
		Name:      wrapperspb.String(" "),
		Extension: wrapperspb.String("csv"),
		Content:   &enrichment.File_Textual{Textual: wrapperspb.String(fileContent)},
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
