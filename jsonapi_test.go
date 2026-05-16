package main

import (
	"context"
	"testing"

	"github.com/gopasspw/gopass/pkg/ctxutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestJSONAPI(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ctx = ctxutil.WithAlwaysYes(ctx, true)

	act := &jsonapiCLI{}

	require.NoError(t, act.listen(ctx, &cli.Command{}))

	b, err := act.getBrowser(ctx, &cli.Command{})
	require.NoError(t, err)
	assert.Equal(t, "chrome", b)
}
