package config

import (
	"sort"

	"github.com/admpub/collate"
)

type SortByAccount []*Account

func (s SortByAccount) Len() int { return len(s) }
func (s SortByAccount) Less(i, j int) bool {
	if s[i].Sort == s[j].Sort {
		return collate.Less(s[i].Options.Title, s[j].Options.Title)
	}
	return s[i].Sort < s[j].Sort
}
func (s SortByAccount) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s SortByAccount) Sort() SortByAccount {
	sort.Sort(s)
	return s
}
func (s SortByAccount) Lite() []*AccountLite {
	r := make([]*AccountLite, len(s))
	for k, v := range s {
		r[k] = v.Lite()
	}
	return r
}
