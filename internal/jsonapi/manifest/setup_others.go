//go:build !windows
// +build !windows

package manifest

import (
	"path/filepath"
	"sort"

	"github.com/gopasspw/gopass/pkg/appdir"
	"golang.org/x/exp/maps"
)

// WrapperName is the name of the gopass wrapper.
var WrapperName = "gopass_wrapper.sh"

// ValidBrowser returns true if the given browser is supported on this platform.
func ValidBrowser(name string) bool {
	_, err := manifestPaths.Local(name)

	return err == nil
}

// ValidBrowsers are all browsers for which the manifest can be currently installed.
func ValidBrowsers() []string {
	keys := maps.Keys(manifestPaths.local)
	sort.Strings(keys)

	return keys
}

// Path returns the manifest file path.
func Path(browser, libpath string, globalInstall bool) (string, error) {
	loc, err := getLocation(browser, libpath, globalInstall)
	if err != nil {
		return "", err
	}

	return filepath.Join(expandHomedir(loc), Name+".json"), nil
}

func expandHomedir(dir string) string {
	if len(dir) < 1 {
		return dir
	}
	if dir[0] != '~' {
		return dir
	}

	return filepath.Join(appdir.UserHome(), dir[1:])
}

// getLocation returns only the manifest path.
func getLocation(browser, libpath string, globalInstall bool) (string, error) {
	if globalInstall {
		return manifestPaths.Global(browser, libpath)
	}

	return manifestPaths.Local(browser)
}
