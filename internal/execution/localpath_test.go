package execution

import (
	"os"
	"path/filepath"
	"testing"
)

// newScriptRoot points scriptRootEnv at a temp dir containing one allowed script,
// and returns the root plus the secret file planted just outside it.
func newScriptRoot(t *testing.T) (root string, secret string) {
	t.Helper()

	base := t.TempDir()
	root = filepath.Join(base, "scripts")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "deploy.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatalf("write allowed script: %v", err)
	}

	// Stands in for .env / id_rsa: a real, readable file the server process owns
	// but that job steps must never be able to name.
	secret = filepath.Join(base, "secret.env")
	if err := os.WriteFile(secret, []byte("GRUNTDECK_MASTER_KEY=hunter2\n"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	t.Setenv(scriptRootEnv, root)
	return root, secret
}

func TestResolveSourcePathAllowsFileInsideRoot(t *testing.T) {
	root, _ := newScriptRoot(t)

	got, err := resolveSourcePath("deploy.sh")
	if err != nil {
		t.Fatalf("expected relative path inside root to resolve, got error: %v", err)
	}
	if want := filepath.Join(root, "deploy.sh"); got != want {
		t.Errorf("resolved to %q, want %q", got, want)
	}

	// The seeded demo jobs use "./name" form, so it must keep working.
	if _, err := resolveSourcePath("./deploy.sh"); err != nil {
		t.Errorf("expected ./deploy.sh to resolve, got error: %v", err)
	}
}

func TestResolveSourcePathRejectsEscapes(t *testing.T) {
	_, secret := newScriptRoot(t)

	cases := map[string]string{
		"parent traversal":   "../secret.env",
		"nested traversal":   "./nested/../../secret.env",
		"deep traversal":     "../../../../../../etc/passwd",
		"absolute secret":    secret,
		"absolute proc":      "/proc/self/environ",
		"absolute etc":       "/etc/passwd",
		"empty":              "",
		"directory not file": ".",
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if got, err := resolveSourcePath(input); err == nil {
				t.Errorf("expected %q to be rejected, but it resolved to %q", input, got)
			}
		})
	}
}

func TestResolveSourcePathRejectsSymlinkOutOfRoot(t *testing.T) {
	root, secret := newScriptRoot(t)

	link := filepath.Join(root, "innocent.sh")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if got, err := resolveSourcePath("innocent.sh"); err == nil {
		t.Errorf("expected symlink escaping root to be rejected, but it resolved to %q", got)
	}
}
