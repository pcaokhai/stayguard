package pricing

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// The schema file is the contract: the hand-written validator may not drift from it.
const schemaFile = "../../../../contracts/pricing/rate-plan.schema.json"

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	abs, err := filepath.Abs(schemaFile)
	if err != nil {
		t.Fatal(err)
	}
	s, err := jsonschema.NewCompiler().Compile(abs)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return s
}

func schemaAccepts(t *testing.T, s *jsonschema.Schema, doc string) bool {
	t.Helper()
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(doc))
	if err != nil {
		return false // not JSON, so not a valid plan
	}
	return s.Validate(inst) == nil
}

func TestRatePlanMatchesSchemaFile_SG101_AC2(t *testing.T) {
	s := compileSchema(t)
	var docs []planCase
	docs = append(docs, planCases(t)...)
	for _, d := range malformedCases {
		docs = append(docs, planCase{name: "malformed " + d, doc: d, want: []FieldError{fe("", "INVALID_JSON")}})
	}
	for _, c := range docs {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseRatePlan([]byte(c.doc))
			goOK, schemaOK := err == nil, schemaAccepts(t, s, c.doc)
			if goOK != schemaOK {
				t.Fatalf("disagree: go accepts=%v schema accepts=%v\n%s", goOK, schemaOK, c.doc)
			}
			if goOK != (c.want == nil) {
				t.Fatalf("table expectation disagrees with both: accepts=%v", goOK)
			}
		})
	}
}

// Documents the one deliberate divergence: JSON Schema "integer" accepts 80000.0 and 8e4,
// the Go validator does not, so a client cannot smuggle a float literal into money.
func TestRatePlanStricterThanSchemaOnIntegerLiterals_SG101_AC2(t *testing.T) {
	s := compileSchema(t)
	for _, c := range strictIntegerCases(t) {
		if _, err := ParseRatePlan([]byte(c.doc)); err == nil {
			t.Errorf("%s: Go must reject", c.name)
		}
		if !schemaAccepts(t, s, c.doc) {
			t.Errorf("%s: schema no longer accepts it; the divergence is gone, fold this row into the main table", c.name)
		}
	}
}
