//go:build !windows
// +build !windows

package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManifest(t *testing.T) {
	t.Parallel()

	_, err := getLocation("foobar", "", false)
	assert.Error(t, err)
}
