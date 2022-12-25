//go:build !windows
// +build !windows

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/gopasspw/gopass-jsonapi/internal/jsonapi/manifest"
	"github.com/gopasspw/gopass/pkg/ctxutil"
	"github.com/gopasspw/gopass/pkg/gopass/api"
	"github.com/gopasspw/gopass/pkg/termio"
	"github.com/urfave/cli/v2"
)

// setup sets up manifest for gopass as native messaging host.
func (s *jsonapiCLI) setup(c *cli.Context) error {
	ctx := ctxutil.WithGlobalFlags(c)
	browser, err := s.getBrowser(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to get browser: %w", err)
	}

	globalInstall, err := s.getGlobalInstall(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to get global flag: %w", err)
	}

	libPath, err := s.getLibPath(ctx, c, browser, globalInstall)
	if err != nil {
		return fmt.Errorf("failed to get lib path: %w", err)
	}

	wrapperPath, err := s.getWrapperPath(ctx, c, api.ConfigDir(), manifest.WrapperName)
	if err != nil {
		return fmt.Errorf("failed to get wrapper path: %w", err)
	}
	wrapperPath = filepath.Join(wrapperPath, manifest.WrapperName)

	manifestPath := c.String("manifest-path")
	if manifestPath == "" {
		p, err := manifest.Path(browser, libPath, globalInstall)
		if err != nil {
			return fmt.Errorf("failed to get manifest path: %w", err)
		}
		manifestPath = p
	}

	wrap, mf, err := manifest.Render(browser, wrapperPath, c.String("gopass-path"), globalInstall)
	if err != nil {
		return fmt.Errorf("failed to render manifest: %w", err)
	}

	if c.Bool("print") {
		fmt.Printf("Native Messaging Setup Preview:\nWrapper Script (%s):\n%s\n\nManifest File (%s):\n%s\n", wrapperPath, string(wrap), manifestPath, string(mf))
	}

	if install, err := termio.AskForBool(ctx, color.BlueString("Install manifest and wrapper?"), true); err != nil || !install {
		return err
	}

	if os.Getenv("GNUPGHOME") != "" {
		fmt.Printf("You seem to have GNUPGHOME set. If you intend to use the path in GNUPGHOME, you need to manually add:\n" +
			"\n  export GNUPGHOME=/path/to/gpg-home\n\n to the wrapper script")
	}

	if err := os.MkdirAll(filepath.Dir(wrapperPath), 0o755); err != nil {
		return fmt.Errorf("failed to create wrapper path: %w", err)
	}

	if err := ioutil.WriteFile(wrapperPath, wrap, 0o755); err != nil {
		return fmt.Errorf("failed to write wrapper script: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return fmt.Errorf("failed to create manifest path: %w", err)
	}

	if err := ioutil.WriteFile(manifestPath, mf, 0o644); err != nil {
		return fmt.Errorf("failed to write manifest file: %w", err)
	}

	return nil
}
