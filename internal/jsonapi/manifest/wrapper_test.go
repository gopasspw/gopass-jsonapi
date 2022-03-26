package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	wrapperGolden = `#!/bin/sh

export PATH="$PATH:$HOME/.nix-profile/bin" # required for Nix
export PATH="$PATH:/usr/local/bin" # required on MacOS/brew
export PATH="$PATH:/usr/local/MacGPG2/bin" # required on MacOS/GPGTools GPGSuite
export GPG_TTY="$(tty)"

if [ -f ~/.gpg-agent-info ] && [ -n "$(pgrep gpg-agent)" ]; then
	source ~/.gpg-agent-info
	export GPG_AGENT_INFO
else
	eval $(gpg-agent --daemon)
fi

gopass-jsonapi listen

exit $?
`
)

func TestWrapperContent(t *testing.T) {
	t.Parallel()

	b, err := getWrapperContent("gopass-jsonapi")
	require.NoError(t, err)
	assert.Equal(t, wrapperGolden, string(b))
}
