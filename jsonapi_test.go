package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/gopasspw/gopass-jsonapi/internal/jsonapi"
	"github.com/gopasspw/gopass/pkg/ctxutil"
	"github.com/gopasspw/gopass/pkg/gopass/apimock"
	"github.com/gopasspw/gopass/pkg/passkey"
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

func TestNativeMessageDisconnect(t *testing.T) {
	t.Setenv("GOPASS_HOMEDIR", t.TempDir())
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	in, browser := io.Pipe()
	defer func() { _ = in.Close() }()
	defer func() { _ = browser.Close() }()
	const msg = `{"type":"passkeyCreate","origin":"https://example.com","options":{"user":{"id":"aWQ","name":"alice"},"challenge":"Yw"}}`
	go func() {
		frame := binary.NativeEndian.AppendUint32(nil, uint32(len(msg)))
		_, _ = browser.Write(append(frame, msg...))
	}()

	var out bytes.Buffer
	api := jsonapi.New(apimock.New(), in, &out, semver.Version{})
	api.Confirm = func(ctx context.Context, _ passkey.ConsentRequest) (bool, error) {
		require.NoError(t, browser.Close())
		<-ctx.Done()

		return false, ctx.Err()
	}

	require.NoError(t, serveNativeMessage(ctx, api))
	assert.Empty(t, out.Bytes(), "listen must not send an error frame after disconnect")
}
