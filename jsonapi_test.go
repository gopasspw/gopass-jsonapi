package main

import (
	"context"
	"testing"

	"github.com/gopasspw/gopass/pkg/ctxutil"
	"github.com/gopasspw/gopass/tests/gptest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONAPI(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = ctxutil.WithAlwaysYes(ctx, true)

	act := &jsonapiCLI{}

	require.NoError(t, act.listen(gptest.CliCtx(ctx, t)))

	b, err := act.getBrowser(ctx, gptest.CliCtx(ctx, t))
	require.NoError(t, err)
	assert.Equal(t, "chrome", b)
}
