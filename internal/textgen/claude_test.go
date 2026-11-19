package textgen

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestModelForRoute(t *testing.T) {
	if got := modelFor(true); got != anthropic.ModelClaudeSonnet5 {
		t.Fatalf("home model = %s", got)
	}
	if got := modelFor(false); got != routeModel || got == anthropic.ModelClaudeSonnet5 {
		t.Fatalf("route model = %s", got)
	}
	if got := effortFor(true); got != anthropic.OutputConfigEffortMedium {
		t.Fatalf("home effort = %s", got)
	}
	if got := effortFor(false); got != anthropic.OutputConfigEffortLow {
		t.Fatalf("route effort = %s", got)
	}
}
