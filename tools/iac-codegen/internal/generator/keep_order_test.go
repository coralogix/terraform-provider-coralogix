package generator

import (
	"strings"
	"testing"
)

// A list that keeps the prior order and whose response item the request does not send pairs its
// items with the request item. Create and Update must send the same fields, and a list that no
// request sends has no request item to pair with.
func TestRequestObject(t *testing.T) {
	field := func(name string) *convField { return &convField{TFName: name} }
	resp := &convObject{Model: "RemoteModel", SDK: "Remote", Fields: []*convField{field("id"), field("name"), field("raw")}}
	create := &convObject{Model: "RemoteModel", SDK: "RemoteCreate", Expand: true, Fields: []*convField{field("name"), field("raw")}}
	replace := &convObject{Model: "RemoteModel", SDK: "RemoteReplace", Expand: true, Fields: []*convField{field("raw"), field("name")}}
	partial := &convObject{Model: "RemoteModel", SDK: "RemotePatch", Expand: true, Fields: []*convField{field("name")}}
	other := &convObject{Model: "OtherModel", SDK: "OtherCreate", Expand: true}
	tests := map[string]struct {
		objects []*convObject
		want    *convObject
		err     string
	}{
		"create":              {[]*convObject{resp, other, create}, create, ""},
		"create and replace":  {[]*convObject{resp, create, replace}, create, ""},
		"different fields":    {[]*convObject{resp, create, partial}, nil, "send different fields"},
		"no request sends it": {[]*convObject{resp, other}, nil, "a list that the request sends"},
	}
	for name, test := range tests {
		got, err := requestObject(&convData{Objects: test.objects}, resp)
		if test.err != "" {
			if err == nil || !strings.Contains(err.Error(), test.err) {
				t.Errorf("%s: err = %v, want %q", name, err, test.err)
			}
			continue
		}
		if err != nil || got != test.want {
			t.Errorf("%s: got %v, %v, want %s", name, got, err, test.want.SDK)
		}
	}
}
