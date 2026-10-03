package acceptance

import (
	"strings"
	"testing"
)

func TestParseAcceptsAFullFile(t *testing.T) {
	f, err := Parse([]byte(`
resource: GlobalRouter
env: [SLACK_INTEGRATION_ID, PD_INTEGRATION_ID]
prerequisites: |
  resource "coralogix_connector" "slack" {
    id = "@{run}-slack"
    value = "@{env.SLACK_INTEGRATION_ID}"
  }
values:
  rules[].targets[].connector_id: coralogix_connector.slack.id
skip: [fallback]
minimal: [routing_labels]
upgradeMinimal: [description]
upgradeFrom: "3.19.0"
`))
	if err != nil {
		t.Fatal(err)
	}
	if f.Resource != "GlobalRouter" || len(f.Env) != 2 || f.UpgradeFrom != "3.19.0" || f.Skip[0] != "fallback" || f.Minimal[0] != "routing_labels" || f.UpgradeMinimal[0] != "description" {
		t.Fatalf("file = %+v", f)
	}
	if got := f.ValuePaths(); len(got) != 1 || got[0] != "rules[].targets[].connector_id" {
		t.Fatalf("value paths = %v", got)
	}
}

func TestParseRejectsWhatCannotWork(t *testing.T) {
	tests := map[string]struct {
		text string
		want string
	}{
		"empty":                {"", "is empty"},
		"unknown key":          {"resource: R\nbogus: 1\n", "bogus"},
		"no resource":          {"env: [A]\n", "resource is required"},
		"bad env name":         {"resource: R\nenv: [lower]\n", "not an environment variable"},
		"env twice":            {"resource: R\nenv: [A, A]\n", "listed twice"},
		"unknown placeholder":  {"resource: R\nprerequisites: x @{foo}\n", "not @{run}"},
		"env not listed":       {"resource: R\nprerequisites: x @{env.MISSING}\n", "not @{run}"},
		"bad value path":       {"resource: R\nvalues:\n  Rules.x: y\n", "not a field path"},
		"empty value":          {"resource: R\nvalues:\n  name: \"\"\n", "is empty"},
		"bad skip path":        {"resource: R\nskip: [Bad]\n", "not a field path"},
		"bad minimal path":     {"resource: R\nminimal: [a.B]\n", "not a field path"},
		"bad upgradeMinimal":   {"resource: R\nupgradeFrom: \"1.0.0\"\nupgradeMinimal: [Bad]\n", "not a field path"},
		"upgradeMinimal alone": {"resource: R\nupgradeMinimal: [description]\n", "upgradeMinimal needs upgradeFrom"},
		"bad version":          {"resource: R\nupgradeFrom: latest\n", "want a release"},
		"backtick":             {"resource: R\nprerequisites: \"a`b\"\n", "backtick"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(test.text))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err = %v, want %q", err, test.want)
			}
		})
	}
}

func TestHCLBraceSyntaxIsNotAPlaceholder(t *testing.T) {
	// HCL uses ${...} and the preset templates use {{...}}. Neither is a placeholder.
	_, err := Parse([]byte("resource: R\nprerequisites: |\n  a = \"${var.x}\" b = \"{{alert.status}}\"\n"))
	if err != nil {
		t.Fatalf("err = %v, want none", err)
	}
}

func TestUpgradeAttributesRoundTrip(t *testing.T) {
	in := &UpgradeAttributes{From: "3.19.0", Attributes: []string{"rules[].name", "name"}}
	data, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	out, err := ParseUpgrade(data)
	if err != nil {
		t.Fatal(err)
	}
	if out.From != "3.19.0" || strings.Join(out.Attributes, ",") != "name,rules[].name" {
		t.Fatalf("read back %+v, want the sorted attributes", out)
	}
	for name, text := range map[string]string{
		"bad from":    "from: latest\nattributes: [name]\n",
		"unknown key": "from: \"3.19.0\"\nother: x\n",
	} {
		if _, err := ParseUpgrade([]byte(text)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
