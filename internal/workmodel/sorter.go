package workmodel

import (
	"sort"
	"strings"
)

// SortUnits orders main numbered units before extras/specials, preserving stable ties.
func SortUnits(items []ResolvedUnit) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].SortKey, items[j].SortKey
		if a.KindRank != b.KindRank {
			return a.KindRank < b.KindRank
		}
		if a.Number != b.Number {
			return a.Number < b.Number
		}
		return strings.Compare(a.Raw, b.Raw) < 0
	})
}
