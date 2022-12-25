package jsonapi

import (
	"context"
	"fmt"
	"io"

	"github.com/blang/semver"
	"github.com/gopasspw/gopass/pkg/ctxutil"
	"github.com/gopasspw/gopass/pkg/gopass"
)

// API type holding store and context
type API struct {
	Store   gopass.Store
	Reader  io.Reader
	Writer  io.Writer
	Version semver.Version
}

// New creates a new instance of the JSON API
func New(s gopass.Store, r io.Reader, w io.Writer, v semver.Version) *API {
	return &API{
		Store:   s,
		Reader:  r,
		Writer:  w,
		Version: v,
	}
}

// ServeMessage a single message
func (api *API) ServeMessage(ctx context.Context) error {
	ctx = ctxutil.WithHidden(ctx, true)

	req, err := readRequest(api.Reader)
	if req == nil || err != nil {
		if err == nil && req == nil {
			err = fmt.Errorf("request message is nil")
		}
		return api.sendErrorResponse(err)
	}

	return api.sendResponse(ctx, req)
}

// sendErrorResponse sends err as JSON response
func (api *API) sendErrorResponse(err error) error {
	return sendJSONResponse(errorResponse{
		Error: err.Error(),
	}, api.Writer)
}
