package filetree

import (
	"slices"
	"sort"
)

type SortOrder int

const (
	ByName SortOrder = iota
	BySizeDesc

	NumSortOrderConventions
)

type OrderStrategy interface {
	orderKeys(files map[string]*FileNode) []string
}

func GetSortOrderStrategy(sortOrder SortOrder) OrderStrategy {
	switch sortOrder {
	case ByName:
		return orderByNameStrategy{}
	case BySizeDesc:
		return orderBySizeDescStrategy{}
	}
	return orderByNameStrategy{}
}

type orderByNameStrategy struct{}

func (orderByNameStrategy) orderKeys(files map[string]*FileNode) []string {
	keys := make([]string, len(files))
	i := 0
	for key := range files {
		keys[i] = key
		i++
	}

	slices.Sort(keys)
	return keys
}

type orderBySizeDescStrategy struct{}

func (orderBySizeDescStrategy) orderKeys(files map[string]*FileNode) []string {
	keys := make([]string, len(files))
	i := 0
	for key := range files {
		keys[i] = key
		i++
	}

	sort.Slice(keys, func(i, j int) bool {
		ki, kj := keys[i], keys[j]
		ni, nj := files[ki], files[kj]
		if ni.CalculateSize() == nj.CalculateSize() {
			return ki < kj
		}
		return ni.CalculateSize() > nj.CalculateSize()
	})

	return keys
}
