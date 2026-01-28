package check

import "sort"

// GetChecks returns all registered checks filtered by mode and categories,
// sorted by (category, name).
func GetChecks(mode string, categories []string) []Check {
	catSet := make(map[string]bool, len(categories))
	for _, c := range categories {
		catSet[c] = true
	}

	var result []Check
	for _, c := range registry {
		// Filter by mode
		if mode != "" && c.Mode() != mode && c.Mode() != "both" {
			continue
		}
		// Filter by categories
		if len(catSet) > 0 && !catSet[c.Category()] {
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
