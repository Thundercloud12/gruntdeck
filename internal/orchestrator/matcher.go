package orchestrator

import "github.com/Thundercloud12/gruntdeck/internal/models"

func MatchTargets(inventory []models.Target, requiredTags []string) []models.Target {
	if len(requiredTags) == 0 {
		return inventory
	}

	var matched []models.Target
	for _, target := range inventory {
		if hasAllTags(target.Tags, requiredTags) {
			matched = append(matched, target)
		}
	}
	return matched
}

func hasAllTags(targetTags, requiredTags []string) bool {
	tagMap := make(map[string]bool)
	for _, t := range targetTags {
		tagMap[t] = true
	}
	for _, r := range requiredTags {
		if !tagMap[r] {
			return false
		}
	}
	return true
}
