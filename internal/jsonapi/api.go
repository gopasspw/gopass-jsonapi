package jsonapi

import (
	"context"
	"io"

	"github.com/blang/semver"
	"github.com/gopasspw/gopass/pkg/ctxutil"
	"github.com/gopasspw/gopass/pkg/gopass"
)

// API type holding store and context.
type API struct {
	Store   gopass.Store
	Reader  io.Reader
	Writer  io.Writer
	Version semver.Version
}

// New creates a new instance of the JSON API.
func New(s gopass.Store, r io.Reader, w io.Writer, v semver.Version) *API {
	return &API{
		Store:   s,
		Reader:  r,
		Writer:  w,
		Version: v,
	}
}

// ServeMessage processes a single message.
func (api *API) ServeMessage(ctx context.Context) error {
	ctx = ctxutil.WithHidden(ctx, true)
	msg, err := readRequest(api.Reader)
	if msg == nil || err != nil {
		return err
	}

	return api.sendResponse(ctx, msg)
}

// SendErrorResponse sends err as JSON response.
func (api *API) SendErrorResponse(err error) error {
	return sendResponse(errorResponse{
		Error: err.Error(),
	}, api.Writer)
}
