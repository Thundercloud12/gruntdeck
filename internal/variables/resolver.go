package variables

import (
	"regexp"
	"strings"

	"github.com/Thundercloud12/gruntdeck/internal/models"
)

var templateRegex = regexp.MustCompile(`\$\{([a-zA-Z0-9_\-\.]+)\}`)

// Resolve parses input template string and replaces tokens using execution context.
func Resolve(input string, execCtx models.ExecutionContext) string {
	if input == "" {
		return ""
	}

	return templateRegex.ReplaceAllStringFunc(input, func(match string) string {
		submatches := templateRegex.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		expression := submatches[1]
		parts := strings.SplitN(expression, ".", 2)
		if len(parts) < 2 {
			return match
		}

		namespace := strings.ToLower(parts[0])
		key := parts[1]

		switch namespace {
		case "job":
			switch strings.ToLower(key) {
			case "id":
				return execCtx.Job.ID
			case "name":
				return execCtx.Job.Name
			}

		case "node":
			switch strings.ToLower(key) {
			case "id":
				return execCtx.Target.ID
			case "host":
				return execCtx.Target.Host
			case "user":
				return execCtx.Target.User
			case "port":
				return execCtx.Target.Port
			}

		case "execution":
			switch strings.ToLower(key) {
			case "id":
				return execCtx.Execution.ID
			case "status":
				return execCtx.Execution.Status
			}

		case "option":
			if val, ok := execCtx.Options[key]; ok {
				return val
			}
			// Fallback check case-insensitive match
			for k, val := range execCtx.Options {
				if strings.EqualFold(k, key) {
					return val
				}
			}

		case "data":
			if val, ok := execCtx.Data[key]; ok {
				return val
			}
		}

		return match
	})
}
