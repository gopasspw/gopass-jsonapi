//go:build !windows

package jsonapi

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/gopasspw/gopass/pkg/gopass/apimock"
	"github.com/gopasspw/gopass/pkg/passkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedPinentry installs a program that speaks the protocol of a pinentry
// and answers a request for confirmation with the given line. Everything it
// is told is written to the returned file.
func scriptedPinentry(t *testing.T, answer string) string {
	t.Helper()

	home := t.TempDir()
	log := filepath.Join(home, "pinentry.log")
	bin := filepath.Join(home, "pinentry")

	script := `#!/bin/sh
echo "OK Pleased to meet you"
while read -r line; do
	echo "$line" >> "` + log + `"
	case "$line" in
		CONFIRM*) ` + answer + ` ;;
		BYE*) echo "OK closing connection"; exit 0 ;;
		*) echo "OK" ;;
	esac
done
`
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o700)) //nolint:gosec
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".gnupg"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gnupg", "gpg-agent.conf"), []byte("pinentry-program "+bin+"\n"), 0o600))

	// the pinentry is looked up in the config of the GnuPG agent
	t.Setenv("HOME", home)
	t.Setenv("GOPASS_HOMEDIR", home)
	t.Setenv("GPG_TTY", "")

	return log
}

func TestPinentryProcess(t *testing.T) {
	req := passkey.ConsentRequest{
		Operation: passkey.OperationAuthenticate,
		RPID:      "example.com",
		Origin:    "https://login.example.com",
		UserName:  "alice",
		Entry:     "passkeys/example.com/alice",
	}

	// confirmed
	log := scriptedPinentry(t, `echo "OK"`)

	consent, err := pinentryConfirm(t.Context(), req)
	require.NoError(t, err)
	assert.True(t, consent)

	told, err := os.ReadFile(log)
	require.NoError(t, err)
	assert.Contains(t, string(told), "SETTITLE gopass\n")
	assert.Contains(t, string(told), "SETOK Confirm\n")
	assert.Contains(t, string(told), "SETCANCEL Cancel\n")
	// line breaks are escaped: the question is a single line of the protocol
	assert.Contains(t, string(told), `SETDESC Sign in to example.com as "alice"?%0A%0APasskey: "passkeys/example.com/alice"%0ARequested by https://login.example.com`+"\n")
	assert.Contains(t, string(told), "CONFIRM\n")

	// cancelled and declined
	for _, answer := range []string{
		`echo "ERR 83886179 Operation cancelled <Pinentry>"`,
		`echo "ERR 83886194 Not confirmed <Pinentry>"`,
	} {
		scriptedPinentry(t, answer)

		consent, err = pinentryConfirm(t.Context(), req)
		require.NoError(t, err, answer)
		assert.False(t, consent, answer)
	}

	// Failed: a dialog that could not be shown leaves the request to
	// another authenticator, one that nobody answered does not.
	for answer, unsuitable := range map[string]bool{
		`echo "ERR 83886142 Timeout <Pinentry>"`:                        false,
		`echo "ERR 83918950 Inappropriate ioctl for device <Pinentry>"`: true,
		`echo "what?"`: true,
		`exit 1`:       true,
	} {
		scriptedPinentry(t, answer)

		consent, err = pinentryConfirm(t.Context(), req)
		require.Error(t, err, answer)
		assert.Equal(t, unsuitable, errors.Is(err, passkey.ErrUnsuitable), "%s: %s", answer, err)
		assert.False(t, consent, answer)
	}

	// A dialog nobody answers is taken down when the ceremony times out.
	// That also works if the pinentry is a wrapper whose child keeps the
	// output open after the wrapper was killed.
	for _, answer := range []string{
		`exec sleep 20`,
		`sleep 20 & wait`,
	} {
		scriptedPinentry(t, answer)

		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)

		start := time.Now()
		consent, err = pinentryConfirm(ctx, req)
		cancel()

		require.ErrorIs(t, err, context.DeadlineExceeded, answer)
		require.NotErrorIs(t, err, passkey.ErrUnsuitable, answer)
		assert.False(t, consent, answer)
		assert.Less(t, time.Since(start), 10*time.Second, answer)
	}

	// A pinentry that answers but does not quit does not hold up the answer.
	log = scriptedPinentry(t, `echo "OK"; trap "" TERM HUP; exec sleep 20`)

	start := time.Now()
	consent, err = pinentryConfirm(t.Context(), req)
	require.NoError(t, err)
	assert.True(t, consent)
	assert.Less(t, time.Since(start), 10*time.Second)

	// It is not asked to quit either: its input just ends.
	told, err = os.ReadFile(log)
	require.NoError(t, err)
	assert.NotContains(t, string(told), "BYE")

	// no pinentry at all
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".gnupg"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gnupg", "gpg-agent.conf"), []byte("pinentry-program "+filepath.Join(home, "missing")+"\n"), 0o600))
	t.Setenv("HOME", home)

	consent, err = pinentryConfirm(t.Context(), req)
	require.ErrorIs(t, err, passkey.ErrUnsuitable)
	assert.Contains(t, err.Error(), "failed to start pinentry")
	assert.False(t, consent)
}

// A browser that gives up on a request closes the connection. The ceremony
// ends, and the pinentry is taken down with it.
func TestPinentryDisconnect(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	scriptedPinentry(t, `echo $$ > "`+pidFile+`"; exec sleep 20`)

	in, hangUp := connect(passkeyCreate)
	defer hangUp()

	store := apimock.New()
	api := New(store, in, &bytes.Buffer{}, semver.MustParse("1.2.3"))

	done := make(chan error, 1)

	go func() { done <- api.ServeMessage(t.Context()) }()

	// the dialog is shown
	require.Eventually(t, func() bool {
		_, err := os.Stat(pidFile)

		return err == nil
	}, 10*time.Second, 10*time.Millisecond)

	hangUp()

	var err error
	select {
	case err = <-done:
	case <-time.After(10 * time.Second):
		require.FailNow(t, "the ceremony did not end when the browser hung up")
	}

	require.ErrorIs(t, err, errDisconnected)
	assert.False(t, passkey.IsFallback(err))

	buf, rerr := os.ReadFile(pidFile)
	require.NoError(t, rerr)

	pid, perr := strconv.Atoi(strings.TrimSpace(string(buf)))
	require.NoError(t, perr)
	require.ErrorIs(t, syscall.Kill(pid, 0), syscall.ESRCH, "the pinentry is gone")

	names, err := store.List(t.Context())
	require.NoError(t, err)
	assert.Empty(t, names)
}
