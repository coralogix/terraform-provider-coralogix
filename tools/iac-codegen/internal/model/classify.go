package model

import "fmt"

// classifyTopLevel returns the behavior of a top-level field. With updateIDInBody,
// the Update call has no id in its path, so the id path parameter of Get is a
// field of the Update body. When Create does not send that field, the server
// assigns the id: the field is computed, and Update sends the value from state.
func classifyTopLevel(p Policy, idParam, field string, inCreate, inUpdate, inGet bool) (Behavior, error) {
	if p.UpdateIDInBody && idParam != "" && field == idParam && !inCreate && inUpdate && inGet {
		return Computed, nil
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
