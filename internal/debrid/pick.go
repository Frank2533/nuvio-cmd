package debrid

// pickFile chooses which resolved file to hand back when a provider returns
// several. If fileIdx is valid, it wins (matching the addon stream's own
// file numbering); otherwise the largest file is assumed to be the actual
// video (skipping samples/extras is a reasonable default, and it degrades
// to "the only file" when there's just one).
func pickFile[T any](items []T, fileIdx *int, sizeOf func(T) int64) T {
	if fileIdx != nil && *fileIdx >= 0 && *fileIdx < len(items) {
		return items[*fileIdx]
	}
	largest := items[0]
	for _, it := range items[1:] {
		if sizeOf(it) > sizeOf(largest) {
			largest = it
		}
	}
	return largest
}
