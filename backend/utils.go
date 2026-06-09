// Package main implements a real-time traffic data server.
package main

import "fmt"

// parseIntField attempts to parse a value as an integer, returning (value, ok).
func parseIntField(v interface{}) (int, bool) {
	switch x := v.(type) {
	case string:
		var n int
		if _, err := fmt.Sscanf(x, "%d", &n); err == nil {
			return n, true
		}
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	}
	return 0, false
}

// pairKey returns the "src:dest" key for a source-destination pair.
func pairKey(src, dest string) string {
	return fmt.Sprintf("%s:%s", src, dest)
}

func Sum(nums []int) int {
	sum := 0
	for _, v := range nums {
		sum += v
	}
	return sum
}
