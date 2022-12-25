package manifest

import (
	"bytes"
	"html/template"
	"os"
	"os/exec"
	"strings"

	"github.com/gopasspw/gopass/pkg/fsutil"
	"github.com/mitchellh/go-homedir"
)

const wrapperTemplate = `#!/bin/sh

export PATH="$PATH:$HOME/.nix-profile/bin" # required for Nix
export PATH="$PATH:/usr/local/bin" # required on MacOS/brew
export PATH="$PATH:/usr/local/MacGPG2/bin" # required on MacOS/GPGTools GPGSuite
export GPG_TTY="$(tty)"

# Uncomment to debug gopass-jsonapi
# export GOPASS_DEBUG_LOG=/tmp/gopass-jsonapi.log

if [ -f ~/.gpg-agent-info ] && [ -n "$(pgrep gpg-agent)" ]; then
	source ~/.gpg-agent-info
	export GPG_AGENT_INFO
else
	eval $(gpg-agent --daemon)
fi

export PATH="$PATH:{{ .Path }}"

{{ .Gopass }} listen

exit $?
`

var isDirFn func(string) bool = fsutil.IsDir

func binsPath() string {
	// required for MacOS with Homebrew
	path := "/usr/local/bin"

	if p := "/opt/homebrew/bin"; isDirFn(p) {
		path = p + ":" + path
	}

	// required for GPGTools on MacOS
	gpgTools := "/usr/local/MacGPG2/bin"
	if isDirFn(gpgTools) {
		path += gpgTools + ":" + path
	}

	return path
}

// Render returns the rendered wrapper and manifest.
func Render(browser, wrapperPath, binPath string, global bool) ([]byte, []byte, error) {
	mf, err := getManifestContent(browser, wrapperPath)
	if err != nil {
		return nil, nil, err
	}

	if binPath == "" {
		binPath = gopassPath(global)
	}
	wrap, err := getWrapperContent(binPath)
	if err != nil {
		return nil, nil, err
	}

	return wrap, mf, nil
}

func getWrapperContent(gopassPath string) ([]byte, error) {
	tmpl, err := template.New("").Parse(wrapperTemplate)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	err = tmpl.Execute(
		buf,
		struct {
			Gopass string
			Path   string
		}{
			Gopass: gopassPath,
			Path:   binsPath(),
		},
	)

	return buf.Bytes(), err
}

func gopassPath(global bool) string {
	if !global {
		if hd, err := homedir.Dir(); err == nil {
			if gpp, err := os.Executable(); err == nil && strings.HasPrefix(gpp, hd) {
				return gpp
			}
		}
	}

	if gpp, err := exec.LookPath("gopass-jsonapi"); err == nil {
		return gpp
	}

	return "gopass-jsonapi"
}
