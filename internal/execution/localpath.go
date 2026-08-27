package execution

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// scriptRootEnv names the directory that job step source_path values are confined to.
const scriptRootEnv = "GRUNTDECK_SCRIPT_DIR"

// defaultScriptDir is the script root used when scriptRootEnv is unset, relative to
// the working directory. It is deliberately a dedicated subdirectory rather than the
// working directory itself, so that .env, .git and known_hosts stay out of reach.
const defaultScriptDir = "scripts"

// resolveSourcePath resolves a job step's source_path against the configured script
// root and rejects anything that escapes it.
//
// source_path names a file on the Gruntdeck server, and any authenticated user can
// author a job step. Left unconfined it reads whatever the server process can read
// (.env, /proc/self/environ, SSH private keys) and ships it to a remote node, so the
// containment check below is a trust boundary, not a convenience.
func resolveSourcePath(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("source_path is empty")
	}

	root, err := scriptRoot()
	if err != nil {
		return "", err
	}

	candidate := input
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}

	// Resolve symlinks before comparing, so a link inside the root that points
	// outside it cannot pass the containment check below.
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("source_path %q is not readable under %s: %w", input, root, err)
	}

	if !isWithin(root, resolved) {
		return "", fmt.Errorf("source_path %q resolves outside the permitted script directory %s", input, root)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("source_path %q is not readable: %w", input, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("source_path %q is a directory, not a file", input)
	}

	return resolved, nil
}

// scriptRoot returns the absolute, symlink-resolved directory that source_path values
// are confined to.
func scriptRoot() (string, error) {
	root := os.Getenv(scriptRootEnv)
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to determine working directory: %w", err)
		}
		root = filepath.Join(wd, defaultScriptDir)
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("invalid %s %q: %w", scriptRootEnv, root, err)
	}

	// The root itself may be reached through a symlink (container mounts, /tmp on
	// macOS); resolve it too so the comparison is like-for-like.
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("script directory %s is not accessible (set %s to override): %w", abs, scriptRootEnv, err)
	}
	return resolved, nil
}

// isWithin reports whether path is root itself or sits underneath it. Both arguments
// must already be absolute and symlink-resolved.
func isWithin(root, path string) bool {
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
