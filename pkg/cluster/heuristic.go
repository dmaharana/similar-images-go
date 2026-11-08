package cluster

import (
	"fmt"
	"strings"
)

// Heuristic defines the rule used to pick the canonical candidate to retain in a duplicate cluster.
type Heuristic string

const (
	HeuristicHighestResolution Heuristic = "resolution"
	HeuristicLargestFileSize   Heuristic = "size"
	HeuristicOldestTimestamp   Heuristic = "oldest"
	HeuristicNewestTimestamp   Heuristic = "newest"
)

// ParseHeuristic parses a string into a Heuristic.
func ParseHeuristic(s string) (Heuristic, error) {
	norm := strings.ToLower(strings.TrimSpace(s))
	switch norm {
	case "resolution", "highest_resolution", "highest-resolution", "res":
		return HeuristicHighestResolution, nil
	case "size", "largest_file_size", "largest-file-size", "file_size":
		return HeuristicLargestFileSize, nil
	case "oldest", "oldest_timestamp", "oldest-timestamp":
		return HeuristicOldestTimestamp, nil
	case "newest", "newest_timestamp", "newest-timestamp":
		return HeuristicNewestTimestamp, nil
	default:
		return "", fmt.Errorf("unknown heuristic: %q (supported: resolution, size, oldest, newest)", s)
	}
}

// PickCanonical returns the index of the recommended candidate to keep.
func PickCanonical(candidates []Candidate, h Heuristic) int {
	if len(candidates) == 0 {
		return 0
	}
	bestIdx := 0
	for i := 1; i < len(candidates); i++ {
		curr := &candidates[i]
		best := &candidates[bestIdx]

		var isBetter bool
		switch h {
		case HeuristicHighestResolution:
			currPixels := curr.Width * curr.Height
			bestPixels := best.Width * best.Height
			if currPixels > bestPixels {
				isBetter = true
			} else if currPixels == bestPixels {
				isBetter = curr.FileSize > best.FileSize
			}
		case HeuristicLargestFileSize:
			if curr.FileSize > best.FileSize {
				isBetter = true
			} else if curr.FileSize == best.FileSize {
				isBetter = curr.Width*curr.Height > best.Width*best.Height
			}
		case HeuristicOldestTimestamp:
			isBetter = curr.Mtime < best.Mtime
		case HeuristicNewestTimestamp:
			isBetter = curr.Mtime > best.Mtime
		default:
			currPixels := curr.Width * curr.Height
			bestPixels := best.Width * best.Height
			isBetter = currPixels > bestPixels
		}

		if isBetter {
			bestIdx = i
		}
	}
	return bestIdx
}
