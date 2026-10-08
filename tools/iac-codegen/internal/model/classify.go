package model

import "fmt"

// classifyField returns the behavior of field. An Update that has no id in its
// path still sends the id in the body. That id is a computed attribute: the
// server assigned it, and Update only echoes it.
func classifyField(p Policy, field string, inCreate, inUpdate, inGet, updateDefault bool) (Behavior, error) {
	if p.UpdateIDInBody && field == "id" && !inCreate && inUpdate && inGet {
		inUpdate = false
	}
	// Create omits the field and stores the declared default. Replace can set it.
	if !inCreate && inUpdate && inGet && updateDefault {
		return Normal, nil
	}
	return Classify(inCreate, inUpdate, inGet)
}

// Classify returns the behavior of a top-level field from where it appears
// (README.md, "Field location"). Any other combination is an error.
func Classify(inCreate, inUpdate, inGet bool) (Behavior, error) {
	switch {
	case inCreate && inUpdate && inGet:
		return Normal, nil
	case inCreate && !inUpdate && inGet:
		return Immutable, nil
	case !inCreate && !inUpdate && inGet:
		return Computed, nil
	}
	return "", fmt.Errorf("unsupported field location: create %t, update %t, get %t", inCreate, inUpdate, inGet)
}
