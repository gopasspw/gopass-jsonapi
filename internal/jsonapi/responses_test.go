package jsonapi

import (
	"testing"

	"github.com/gopasspw/gopass/pkg/gopass"
	"github.com/stretchr/testify/assert"
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
			assert.Equal(t, tc.Out, a.getUsername(tc.Name, tc.Sec), "Wrong Username")
		})
	}
}
