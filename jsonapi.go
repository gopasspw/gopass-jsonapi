package main

import (
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/gopasspw/gopass-jsonapi/internal/jsonapi"
	"github.com/gopasspw/gopass/pkg/ctxutil"
	"github.com/gopasspw/gopass/pkg/gopass"
	"github.com/urfave/cli/v2"
)

var (
	stdin  = os.Stdin
	stdout = os.Stdout
)

type jsonapiCLI struct {
	gp gopass.Store
}

// listen reads a json message on stdin and responds on stdout.
func (s *jsonapiCLI) listen(c *cli.Context) error {
	ctx := ctxutil.WithGlobalFlags(c)

	version, err := semver.Parse(strings.TrimPrefix(c.App.Version, "v"))
	if err != nil {
		version = semver.Version{}
	}

	api := jsonapi.New(s.gp, stdin, stdout, version)
	if err := api.ServeMessage(ctx); err != nil {
		return api.SendErrorResponse(err)
	}

	return nil
}
