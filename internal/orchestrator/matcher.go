package orchestrator

import "github.com/Thundercloud12/gruntdeck/internal/config"


func MatchTargets(inventory []config.Target, requiredTags []string) []config.Target {
	if len(requiredTags) == 0 {
		return inventory
	}

	var matched []config.Target
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