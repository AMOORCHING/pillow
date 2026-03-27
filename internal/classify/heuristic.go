package classify

import (
	"path/filepath"
	"strings"
)

// Classify returns the irreversibility level and reason for a tool call.
// Returns level: "none" | "warn" | "block" and a human-readable reason.
func Classify(toolName string, toolInput map[string]any) (level string, reason string) {
	switch toolName {
	case "Write":
		return classifyWrite(toolInput)
	case "Edit":
		return classifyEdit(toolInput)
	case "Bash":
		return classifyBash(toolInput)
	default:
		return "none", ""
	}
}

func classifyWrite(input map[string]any) (string, string) {
	path, _ := input["file_path"].(string)
	if path == "" {
		return "none", ""
	}

	// Block: sensitive file patterns
	if matchesSensitivePath(path) {
		return "block", "sensitive file: " + filepath.Base(path)
	}

	return "none", ""
}

func classifyEdit(input map[string]any) (string, string) {
	path, _ := input["file_path"].(string)
	if path == "" {
		return "none", ""
	}

	if matchesSensitivePath(path) {
		return "warn", "editing sensitive file: " + filepath.Base(path)
	}

	return "none", ""
}

func classifyBash(input map[string]any) (string, string) {
	cmd, _ := input["command"].(string)
	if cmd == "" {
		return "none", ""
	}

	// Normalize for matching
	lower := strings.ToLower(cmd)
	parts := strings.Fields(lower)
	if len(parts) == 0 {
		return "none", ""
	}

	// Block: destructive commands
	base := filepath.Base(parts[0])
	switch base {
	case "rm":
		if containsFlag(parts, "-r", "-rf", "-fr") {
			return "block", "recursive delete"
		}
		return "warn", "file deletion"
	case "mv":
		return "warn", "file move/rename"
	case "chmod", "chown":
		return "warn", "permission change"
	}

	// Block: destructive SQL
	for _, keyword := range []string{"drop ", "alter ", "truncate ", "delete from "} {
		if strings.Contains(lower, keyword) {
			return "block", "destructive SQL: " + strings.TrimSpace(keyword)
		}
	}

	// Warn: potentially dangerous piped commands
	if strings.Contains(cmd, "|") && (strings.Contains(lower, "xargs") && strings.Contains(lower, "rm")) {
		return "block", "piped delete"
	}

	return "none", ""
}

func matchesSensitivePath(path string) bool {
	patterns := []string{
		"**/migrations/**",
		"**/config.*",
		"**/.env*",
		"**/docker-compose*",
		"**/Makefile",
		"**/*.lock",
		"**/schema.*",
		"**/Dockerfile*",
	}

	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		// Also check against just the relative parts
		base := filepath.Base(path)
		dir := filepath.Base(filepath.Dir(path))

		// Check if in migrations directory
		if strings.Contains(path, "/migrations/") || strings.Contains(path, "/migration/") {
			return true
		}
		// Check base name patterns
		basePatterns := []string{
			".env", ".env.*",
			"docker-compose.yml", "docker-compose.yaml",
			"Makefile",
			"Dockerfile",
		}
		for _, bp := range basePatterns {
			if matched, _ := filepath.Match(bp, base); matched {
				return true
			}
		}
		// Check extension patterns
		extPatterns := []string{"*.lock"}
		for _, ep := range extPatterns {
			if matched, _ := filepath.Match(ep, base); matched {
				return true
			}
		}
		// Check config files
		if matched, _ := filepath.Match("config.*", base); matched {
			return true
		}
		if matched, _ := filepath.Match("schema.*", base); matched {
			return true
		}
		_ = dir
	}

	return false
}

func containsFlag(parts []string, flags ...string) bool {
	for _, part := range parts {
		for _, flag := range flags {
			if part == flag {
				return true
			}
		}
	}
	return false
}
