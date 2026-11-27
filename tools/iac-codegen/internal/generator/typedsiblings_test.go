package generator

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
)

func fieldNames(fields []*model.Field) []string {
	var out []string
	for _, f := range fields {
		out = append(out, f.Name)
	}
	return out
}

// A oneOf has only arms. An object with one oneOf group has the arms of the group, and its other
// fields are plain attributes next to them.
func TestTypedArmFieldsSplitsTheArmsFromTheSiblings(t *testing.T) {
	fields := []*model.Field{{Name: "metadata"}, {Name: "tag"}, {Name: "note"}}
	for _, tc := range []struct {
		name         string
		typ          *model.Type
		arms, others []string
	}{
		{"oneOf", &model.Type{Kind: model.OneOf, Fields: fields}, []string{"metadata", "tag", "note"}, nil},
		{"object without group", &model.Type{Kind: model.Object, Fields: fields}, []string{"metadata", "tag", "note"}, nil},
		{"object with group", &model.Type{Kind: model.Object, Fields: fields, Groups: []model.OneOfGroup{{Arms: []string{"metadata", "tag"}}}}, []string{"metadata", "tag"}, []string{"note"}},
	} {
		arms, others := typedArmFields(tc.typ)
		if got := fieldNames(arms); !slices.Equal(got, tc.arms) {
			t.Errorf("%s: arms = %v, want %v", tc.name, got, tc.arms)
		}
		if got := fieldNames(others); !slices.Equal(got, tc.others) {
			t.Errorf("%s: siblings = %v, want %v", tc.name, got, tc.others)
		}
	}
}

// A new field next to an inlined typedArms object, which carries the oneOf group of its arms, is
// no arm: the object stays one type and its merged fields, and the new field is an attribute of its
// own. The check, which has no SDK, accepts it with no line in the overrides.
func TestAFieldBesideAnInlinedTypedObjectIsASibling(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "typedthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	span := "        note: {type: string, x-coralogix-presence: true}\n"
	spec = []byte(replaceOnce(t, string(spec), span, span+"        other: {type: string, x-coralogix-presence: true}\n"))
	dir := t.TempDir()
	path := filepath.Join(dir, "typedthing.yaml")
	if err := os.WriteFile(path, spec, 0o644); err != nil {
		t.Fatal(err)
	}
	err = Check(CheckOptions{Resource: "TypedThing", OpenAPIPath: path, OverridesPath: filepath.Join("testdata", "typedthing-overrides.yaml")})
	if err != nil {
		t.Fatalf("check = %v", err)
	}
}
