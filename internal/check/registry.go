package check

import "sort"

// GetChecks returns all registered checks filtered by mode, categories,
// exclude list, and include-only list. Sorted by (category, name).
func GetChecks(mode string, categories []string, exclude []string, includeOnly []string) []Check {
	catSet := make(map[string]bool, len(categories))
	for _, c := range categories {
		catSet[c] = true
	}

	exclSet := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		exclSet[e] = true
	}

	inclSet := make(map[string]bool, len(includeOnly))
	for _, i := range includeOnly {
		inclSet[i] = true
	}

	var result []Check
	for _, c := range registry {
		if mode != "" && c.Mode() != mode && c.Mode() != "both" {
			continue
		}
		if len(catSet) > 0 && !catSet[c.Category()] {
			continue
		}
		if len(inclSet) > 0 {
			if !inclSet[c.Name()] {
				continue
			}
		} else if exclSet[c.Name()] {
			continue
		}
		result = append(result, c)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Category() != result[j].Category() {
			return result[i].Category() < result[j].Category()
		}
		return result[i].Name() < result[j].Name()
	})

	return result
}
