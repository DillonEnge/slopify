package template

import (
	"strconv"
	"testing"
)

func TestQueueMatchesGeneration(t *testing.T) {
	home := queuePosition("/")
	if home < 8 || home > 14 {
		t.Fatalf("home position = %d", home)
	}
	if queueSlot("/") != 6 || queueHoldSecs("/") != home*6 {
		t.Fatalf("home hold = %d slot = %d", queueHoldSecs("/"), queueSlot("/"))
	}
	if got := queueAhead("/"); got != strconv.Itoa(home-1) {
		t.Fatalf("ahead = %s", got)
	}

	route := queuePosition("/play")
	if route < 3 || route > 6 {
		t.Fatalf("route position = %d", route)
	}
	if queueSlot("/play") != 4 || queueHoldSecs("/play") != route*4 {
		t.Fatalf("route hold = %d", queueHoldSecs("/play"))
	}
}
