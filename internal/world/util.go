package world

import (
	"sort"
	"strconv"
)

func itoa(n int) string { return strconv.Itoa(n) }

func sortStrings(s []string) { sort.Strings(s) }

// sortStringsBy sorts a slice with a less function, for small listings.
func sortStringsBy[T any](s []T, less func(i, j int) bool) { sort.Slice(s, less) }
