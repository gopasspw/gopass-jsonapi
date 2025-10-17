package jsonapi

import (
	"fmt"
	"testing"

	"github.com/gopasspw/gopass/pkg/gopass"
	"github.com/gopasspw/gopass/pkg/gopass/apimock"
	"github.com/stretchr/testify/require"
)

func TestGetUsername(t *testing.T) {
	t.Parallel()

	a := &API{}
	for _, tc := range []struct {
		Name string
		Sec  gopass.Secret
		Out  string
	}{
		{
			Name: "some/fixed/yamlother",
			Sec:  newSec(t, "thesecret\n---\nother: meh"),
			Out:  "yamlother",
		},
		{
			Name: "some/key/withaname",
			Sec:  newSec(t, "thesecret\n---\nlogin: foo"),
			Out:  "foo",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.Out, a.getUsername(tc.Name, tc.Sec), "Wrong Username")
		})
	}
}

func TestGetPassword(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("without referencing - simple password", func(t *testing.T) {
		t.Parallel()

		store := apimock.New()
		api := &API{Store: store}

		// Create a simple secret without reference
		sec := newSec(t, "my-simple-password")

		password, err := api.getPassword(ctx, sec)
		require.NoError(t, err)
		require.Equal(t, "my-simple-password", password)
	})

	t.Run("without referencing - password with yaml metadata", func(t *testing.T) {
		t.Parallel()

		store := apimock.New()
		api := &API{Store: store}

		// Create a secret with yaml metadata but no reference
		sec := newSec(t, "complex-password\n---\nlogin: username\nemail: user@example.com")

		password, err := api.getPassword(ctx, sec)
		require.NoError(t, err)
		require.Equal(t, "complex-password", password)
	})

	t.Run("with single level referencing", func(t *testing.T) {
		t.Parallel()

		store := apimock.New()
		api := &API{Store: store}

		// Create the target secret that will be referenced
		targetSec := newSec(t, "target-password")
		err := store.Set(ctx, "target/secret", targetSec)
		require.NoError(t, err)

		// Create a secret that references the target
		refSec := newSec(t, "gopass://target/secret")

		password, err := api.getPassword(ctx, refSec)
		require.NoError(t, err)
		require.Equal(t, "target-password", password)
	})

	t.Run("with multi-level referencing", func(t *testing.T) {
		t.Parallel()

		store := apimock.New()
		api := &API{Store: store}

		// Create the final target secret
		finalSec := newSec(t, "final-password")
		err := store.Set(ctx, "final/secret", finalSec)
		require.NoError(t, err)

		// Create intermediate reference
		intermediateSec := newSec(t, "gopass://final/secret")
		err = store.Set(ctx, "intermediate/secret", intermediateSec)
		require.NoError(t, err)

		// Create first level reference
		firstSec := newSec(t, "gopass://intermediate/secret")

		password, err := api.getPassword(ctx, firstSec)
		require.NoError(t, err)
		require.Equal(t, "final-password", password)
	})

	t.Run("with deep referencing chain", func(t *testing.T) {
		t.Parallel()

		store := apimock.New()
		api := &API{Store: store}

		// Create a chain of 5 references (should work as it's under the limit of 10)
		finalSec := newSec(t, "deep-password")
		err := store.Set(ctx, "level5", finalSec)
		require.NoError(t, err)

		for i := 4; i >= 1; i-- {
			refSec := newSec(t, fmt.Sprintf("gopass://level%d", i+1))
			err = store.Set(ctx, fmt.Sprintf("level%d", i), refSec)
			require.NoError(t, err)
		}

		// Get password from the first level
		firstSec := newSec(t, "gopass://level1")

		password, err := api.getPassword(ctx, firstSec)
		require.NoError(t, err)
		require.Equal(t, "deep-password", password)
	})

	t.Run("recursion depth limit exceeded", func(t *testing.T) {
		t.Parallel()

		store := apimock.New()
		api := &API{Store: store}

		// Create a chain of 11 references (exceeds the limit of 10)
		finalSec := newSec(t, "too-deep-password")
		err := store.Set(ctx, "level11", finalSec)
		require.NoError(t, err)

		for i := 10; i >= 1; i-- {
			refSec := newSec(t, fmt.Sprintf("gopass://level%d", i+1))
			err = store.Set(ctx, fmt.Sprintf("level%d", i), refSec)
			require.NoError(t, err)
		}

		// Get password from the first level - should fail
		firstSec := newSec(t, "gopass://level1")

		password, err := api.getPassword(ctx, firstSec)
		require.Error(t, err)
		require.Contains(t, err.Error(), "too depth")
		require.Empty(t, password)
	})

	t.Run("circular reference protection", func(t *testing.T) {
		t.Parallel()

		store := apimock.New()
		api := &API{Store: store}

		// Create circular reference: secret1 -> secret2 -> secret1
		sec1 := newSec(t, "gopass://circular/secret2")
		err := store.Set(ctx, "circular/secret1", sec1)
		require.NoError(t, err)

		sec2 := newSec(t, "gopass://circular/secret1")
		err = store.Set(ctx, "circular/secret2", sec2)
		require.NoError(t, err)

		// Should hit the depth limit
		password, err := api.getPassword(ctx, sec1)
		require.Error(t, err)
		require.Contains(t, err.Error(), "too depth")
		require.Empty(t, password)
	})

	t.Run("reference to non-existent secret", func(t *testing.T) {
		t.Parallel()

		store := apimock.New()
		api := &API{Store: store}

		// Create a secret that references a non-existent target
		refSec := newSec(t, "gopass://does/not/exist")

		password, err := api.getPassword(ctx, refSec)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get secret")
		require.Empty(t, password)
	})
}
