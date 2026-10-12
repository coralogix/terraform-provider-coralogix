package model

import "fmt"

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
