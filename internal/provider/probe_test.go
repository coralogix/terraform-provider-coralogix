package provider

import (
	"testing"
)

func TestPrivateShapeProbe(t *testing.T) {
	h := newUserHarness(t)
	s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
	t.Logf("private bytes: %s", string(h.private))
	_ = s
}
