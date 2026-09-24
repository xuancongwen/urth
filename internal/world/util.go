package world

import (
	"sort"
	"strconv"
)

func itoa(n int) string { return strconv.Itoa(n) }

func sortStrings(s []string) { sort.Strings(s) }
