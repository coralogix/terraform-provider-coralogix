package generator

import (
	"bytes"
	"math/big"
	"strconv"

	"go.yaml.in/yaml/v4"
)

// The iac module reads only inclusive numeric bounds: it rejects exclusiveMinimum and
// exclusiveMaximum, and the Terraform validators of a number are AtLeast and AtMost. A contract
// writes "greater than 0" as an exclusive bound, so the generator would refuse it.
//
// When the values of a field are the multiples of a step, an exclusive bound has an exact
// inclusive form: the least multiple above the bound, or the greatest multiple below it. The step
// is multipleOf, or 1 for an integer. The inclusive form accepts exactly the same values:
//
//	type: number, multipleOf: 0.001, exclusiveMinimum: 0   ->   minimum: 0.001
//	type: integer, exclusiveMaximum: 10                    ->   maximum: 9
//	type: integer, minimum: 5, exclusiveMinimum: true      ->   minimum: 6   (OpenAPI 3.0 form)
//
// A number with no multipleOf has no such form: the values next to the bound are not discrete.
// The generator leaves its bound as written, and the iac module reports it as before.

// exclusiveBoundsExact returns data with each exclusive numeric bound that has an exact
// inclusive form replaced by it. A contract with none is returned as it is, byte for byte.
func exclusiveBoundsExact(data []byte) ([]byte, error) {
	if !bytes.Contains(data, []byte("exclusiveM")) {
		return data, nil
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return data, nil // model.Load reports a document that does not parse.
	}
	changed := false
	var walk func(*yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode && rewriteExclusiveBounds(n) {
			changed = true
		}
		for _, child := range n.Content {
			walk(child)
		}
	}
	walk(&root)
	if !changed {
		return data, nil
	}
	return yaml.Marshal(&root)
}

// rewriteExclusiveBounds replaces the exclusive bounds of the schema m that have an exact
// inclusive form, and reports whether it changed m.
func rewriteExclusiveBounds(m *yaml.Node) bool {
	typ := mapValue(m, "type")
	if typ == nil || typ.Kind != yaml.ScalarNode || typ.Value != "number" && typ.Value != "integer" {
		return false
	}
	step := stepOf(m, typ.Value)
	changedMin := rewriteBound(m, "exclusiveMinimum", "minimum", step, true)
	changedMax := rewriteBound(m, "exclusiveMaximum", "maximum", step, false)
	return changedMin || changedMax
}

// stepOf is the distance between two values that the schema accepts: its multipleOf, 1 for an
// integer, and nil when the values are not discrete. An integer with a multipleOf accepts the
// integers that are multiples of it. Those are the multiples of the numerator of multipleOf in
// lowest terms: for 0.5 (1/2) every integer, for 1.5 (3/2) the multiples of 3, for 2 the
// multiples of 2.
func stepOf(m *yaml.Node, typ string) *big.Rat {
	node := mapValue(m, "multipleOf")
	if node == nil {
		if typ == "integer" {
			return big.NewRat(1, 1)
		}
		return nil
	}
	multiple, ok := ratOf(node)
	if !ok || multiple.Sign() <= 0 {
		return nil
	}
	if typ == "integer" {
		return new(big.Rat).SetInt(multiple.Num())
	}
	return multiple
}

// excludesEdge reports whether the bound, read as a float64 as the validators do, is still beyond
// the exclusive edge. A step finer than float64 precision at the edge rounds the bound back onto
// the edge, and the inclusive form would then accept the value that the contract excludes.
func excludesEdge(bound, edge *big.Rat, lower bool) bool {
	f, _ := bound.Float64()
	rounded := new(big.Rat).SetFloat64(f)
	if rounded == nil { // infinite
		return false
	}
	if lower {
		return rounded.Cmp(edge) > 0
	}
	return rounded.Cmp(edge) < 0
}

// rewriteBound turns the exclusive bound of m into the inclusive bound that holds the same values.
// lower is true for a lower bound. It changes nothing when there is no exact form.
func rewriteBound(m *yaml.Node, exclusiveKey, inclusiveKey string, step *big.Rat, lower bool) bool {
	exclusive := mapValue(m, exclusiveKey)
	inclusive := mapValue(m, inclusiveKey)
	if exclusive == nil || exclusive.Kind != yaml.ScalarNode {
		return false
	}
	if exclusive.Tag == "!!bool" {
		// OpenAPI 3.0: true makes minimum or maximum exclusive. false changes nothing.
		if exclusive.Value == "false" {
			removeKey(m, exclusiveKey)
			return true
		}
		if exclusive.Value != "true" || inclusive == nil {
			return false
		}
		return replaceBound(m, exclusiveKey, inclusiveKey, inclusive, step, lower, false)
	}
	return replaceBound(m, exclusiveKey, inclusiveKey, exclusive, step, lower, true)
}

// replaceBound sets the inclusive bound to the nearest multiple of step inside the exclusive
// bound at the node `bound`. When the schema has an inclusive bound too, and its form is the
// OpenAPI 3.1 one, the tighter of the two stays.
func replaceBound(m *yaml.Node, exclusiveKey, inclusiveKey string, bound *yaml.Node, step *big.Rat, lower, keepInclusive bool) bool {
	if step == nil {
		return false
	}
	edge, ok := ratOf(bound)
	if !ok {
		return false
	}
	quotient := new(big.Rat).Quo(edge, step)
	// The least integer above the quotient for a lower bound, the greatest integer below it for an
	// upper bound: a quotient that is a whole number moves by one, any other by its floor or ceiling.
	whole := new(big.Int).Div(quotient.Num(), quotient.Denom()) // floor: the divisor is positive
	if !lower && !quotient.IsInt() {
		whole.Add(whole, big.NewInt(1)) // ceiling
	}
	if lower {
		whole.Add(whole, big.NewInt(1))
	} else {
		whole.Sub(whole, big.NewInt(1))
	}
	result := new(big.Rat).Mul(new(big.Rat).SetInt(whole), step)
	if !excludesEdge(result, edge, lower) {
		return false
	}
	if existing := mapValue(m, inclusiveKey); keepInclusive && existing != nil && existing != bound {
		if current, ok := ratOf(existing); ok && (lower && current.Cmp(result) > 0 || !lower && current.Cmp(result) < 0) {
			result = current
		}
	}
	text, tag := numberText(result)
	if existing := mapValue(m, inclusiveKey); existing != nil {
		existing.Value, existing.Tag, existing.Style = text, tag, 0
	} else {
		m.Content = append(m.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: inclusiveKey},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: text})
	}
	removeKey(m, exclusiveKey)
	return true
}

func ratOf(n *yaml.Node) (*big.Rat, bool) {
	if n.Kind != yaml.ScalarNode || n.Tag != "!!int" && n.Tag != "!!float" {
		return nil, false
	}
	r, ok := new(big.Rat).SetString(n.Value)
	return r, ok
}

// numberText writes r as the shortest decimal that reads back as the same float64.
func numberText(r *big.Rat) (string, string) {
	if r.IsInt() {
		return r.Num().String(), "!!int"
	}
	f, _ := r.Float64()
	return strconv.FormatFloat(f, 'f', -1, 64), "!!float"
}

func removeKey(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}
