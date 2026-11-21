package jsonapi

import (
	"context"
	"io"

	"github.com/blang/semver"
	"github.com/gopasspw/gopass/pkg/ctxutil"
	"github.com/gopasspw/gopass/pkg/gopass"
	"github.com/gopasspw/gopass/pkg/passkey"
)

// API type holding store and context.
type API struct {
	Store   gopass.Store
	Reader  io.Reader
	Writer  io.Writer
	Version semver.Version
	// Confirm asks the user to approve the creation or the use of a
	// passkey. Without it no passkey is created or used. A confirmation is
	// a test of user presence, see passkey.ConsentByConfirmation.
	Confirm passkey.ConfirmFunc
}

// New creates a new instance of the JSON API.
func New(s gopass.Store, r io.Reader, w io.Writer, v semver.Version) *API {
	return &API{
		Store:   s,
		Reader:  r,
		Writer:  w,
		Version: v,
		Confirm: pinentryConfirm,
	}
}

// ServeMessage processes a single message. For passkey creation and
// authentication on streaming input, it consumes any remaining input to
// detect browser disconnects. Do not call it in a loop on that input.
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
	return sendResponse(errorMessage(err), api.Writer)
}
