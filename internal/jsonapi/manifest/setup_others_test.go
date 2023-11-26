//go:build !windows
// +build !windows

package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManifest(t *testing.T) {
	t.Parallel()

	_, err := getLocation("foobar", "", false)
	require.Error(t, err)
}
