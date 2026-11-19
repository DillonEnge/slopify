package template

import "fmt"

// A slot is one generation. Home (Sonnet) is slower than a Haiku route, so the
// queue depth and the hold are the same wait, not two unrelated numbers.
func queueSlot(path string) int {
	if path == "/" {
		return 6
	}
	return 4
}

func queueSlotLabel(path string) string {
	return fmt.Sprintf("%d", queueSlot(path))
}

func queueHash(path string) uint32 {
	var n uint32 = 2166136261
	for _, r := range path {
		n ^= uint32(r)
		n *= 16777619
	}
	return n
}

func queuePosition(path string) int {
	n := queueHash(path)
	if path == "/" {
		return 8 + int(n%7)
	}
	return 3 + int(n%4)
}

func queueHoldSecs(path string) int {
	return queuePosition(path) * queueSlot(path)
}

func queuePosLabel(path string) string {
	return fmt.Sprintf("%d", queuePosition(path))
}

func queueAhead(path string) string {
	return fmt.Sprintf("%d", queuePosition(path)-1)
}

func queueHold(path string) string {
	secs := queueHoldSecs(path)
	return fmt.Sprintf("%02d:%02d", secs/60, secs%60)
}

func queueNode(path string) string {
	return fmt.Sprintf("edge-%02d", 1+queueHash(path)%12)
}
