package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/gopasspw/gopass-jsonapi/internal/jsonapi/manifest"
	"github.com/gopasspw/gopass/pkg/termio"
	"github.com/urfave/cli/v2"
)

func (s *jsonapiCLI) getBrowser(ctx context.Context, c *cli.Context) (string, error) {
	browser := c.String("browser")
	if browser != "" {
		return browser, nil
	}

	browser, err := termio.AskForString(ctx, color.BlueString("For which browser do you want to install gopass native messaging? [%s]", strings.Join(manifest.ValidBrowsers(), ",")), manifest.DefaultBrowser)
	if err != nil {
		return "", fmt.Errorf("failed to ask for user input: %w", err)
	}
	if !manifest.ValidBrowser(browser) {
		return "", fmt.Errorf("%s not one of %s", browser, strings.Join(manifest.ValidBrowsers(), ","))
	}

	return browser, nil
}

func (s *jsonapiCLI) getGlobalInstall(ctx context.Context, c *cli.Context) (bool, error) {
	if !c.IsSet("global") {
		return termio.AskForBool(ctx, color.BlueString("Install for all users? (might require sudo gopass)"), false)
	}

	return c.Bool("global"), nil
}

func (s *jsonapiCLI) getLibPath(ctx context.Context, c *cli.Context, browser string, global bool) (string, error) {
	if !c.IsSet("libpath") && runtime.GOOS == "linux" && (browser == "firefox" || browser == "librewolf") && global {
		return termio.AskForString(ctx, color.BlueString("What is your lib path?"), "/usr/lib")
	}

	return c.String("libpath"), nil
}

func (s *jsonapiCLI) getWrapperPath(ctx context.Context, c *cli.Context, defaultWrapperPath string, wrapperName string) (string, error) {
	if path := c.String("path"); path != "" {
		return path, nil
	}

	path, err := termio.AskForString(ctx, color.BlueString("In which path should %s be installed?", wrapperName), defaultWrapperPath)
	if err != nil {
		return "", fmt.Errorf("failed to ask for user input: %w", err)
	}

	return path, nil
}
