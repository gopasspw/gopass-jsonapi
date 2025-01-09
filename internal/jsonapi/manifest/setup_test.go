package manifest

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	idf := isDirFn
	defer func() {
		isDirFn = idf
	}()

	isDirFn = func(_ string) bool {
		return false
	}

	// windows: C:\Users\johndoe\AppData...
	// *nix: /tmp
	binDir := os.TempDir()

	manifestGolden := `{
    "name": "com.justwatch.gopass",
    "description": "Gopass wrapper to search and return passwords",
    "path": "` + strings.Replace(binDir, "\\", "\\\\", -1) + `",
    "type": "stdio",
    "allowed_origins": [
        "chrome-extension://kkhfnlkhiapbiehimabddjbimfaijdhk/"
    ]
}`
	w, m, err := Render("chrome", binDir, "gopass-jsonapi", true)
	require.NoError(t, err)
	assert.Equal(t, wrapperGolden, string(w))
	assert.Equal(t, manifestGolden, string(m))
}

func TestValidBrowser(t *testing.T) {
	t.Parallel()

	for _, b := range []string{"chrome", "chromium", "firefox"} {
		assert.True(t, ValidBrowser(b))
	}
}

func TestValidBrowsers(t *testing.T) {
	t.Parallel()

	var validBrowsers []string

	switch runtime.GOOS {
	case "darwin": // macOS
		validBrowsers = []string{"arc", "brave", "chrome", "chromium", "firefox", "iridium", "slimjet", "vivaldi"}
	case "windows": // Windows
		validBrowsers = []string{"chrome", "chromium", "firefox"}
	case "linux": // Linux
		validBrowsers = []string{"brave", "chrome", "chromium", "firefox", "iridium", "slimjet", "vivaldi"}
	default: // Fallback, not suppoerted OS
		t.Fatalf("Unsupported OS: %s", runtime.GOOS)
	}

	assert.Equal(t, validBrowsers, ValidBrowsers())
}
