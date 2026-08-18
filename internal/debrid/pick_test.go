package debrid

import "testing"

func TestPickFile(t *testing.T) {
	items := []int64{10, 500, 20}
	sizeOf := func(v int64) int64 { return v }

	if got := pickFile(items, nil, sizeOf); got != 500 {
		t.Errorf("pickFile(nil idx) = %d, want largest (500)", got)
	}

	idx := 0
	if got := pickFile(items, &idx, sizeOf); got != 10 {
		t.Errorf("pickFile(idx=0) = %d, want 10", got)
	}

	outOfRange := 99
	if got := pickFile(items, &outOfRange, sizeOf); got != 500 {
		t.Errorf("pickFile(out-of-range idx) = %d, want fallback to largest (500)", got)
	}
}
