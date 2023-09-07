package manifest

import (
	"fmt"
	"path/filepath"
	"runtime"
)

type manifestPath struct {
	local  map[string]string
	global map[string]string
}

func (m manifestPath) Global(browser, libpath string) (string, error) {
	path, err := lookup(m.global, browser)
	if err != nil {
		return "", err
	}

	if (browser == "firefox" || browser == "librewolf") && libpath != "" {
		return filepath.Join(libpath, path), nil
	}

	return path, nil
}

func (m manifestPath) Local(browser string) (string, error) {
	return lookup(m.local, browser)
}

func lookup(bm map[string]string, browser string) (string, error) {
	if len(bm) < 1 {
		return "", fmt.Errorf("platform %s is currently not supported", runtime.GOOS)
	}

	if sv, found := bm[browser]; found {
		return sv, nil
	}

	if sv, found := bm["default"]; found {
		return sv, nil
	}

	return "", fmt.Errorf("browser %s on %s is currently not supported", browser, runtime.GOOS)
}
