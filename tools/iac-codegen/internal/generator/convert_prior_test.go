package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetFlattenTemplateMatchesPriorByIdentity(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("templates", "convert.go.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "matchPriorSetItem") {
		t.Fatal("set flatten with NeedsPrior must match prior items by identity, not slice index")
	}
}
