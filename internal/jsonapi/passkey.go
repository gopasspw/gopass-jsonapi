package jsonapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gopasspw/gopass/pkg/gopass"
	"github.com/gopasspw/gopass/pkg/passkey"
)

// passkeyOrigin describes the document a WebAuthn request comes from. It
// must be determined by the browser extension, never taken from the page.
type passkeyOrigin struct {
	Origin      string `json:"origin"`
	CrossOrigin bool   `json:"cross_origin"`
	TopOrigin   string `json:"top_origin"`
}

func (o passkeyOrigin) origin() passkey.Origin {
	return passkey.Origin{
		Origin:      o.Origin,
		CrossOrigin: o.CrossOrigin,
		TopOrigin:   o.TopOrigin,
	}
}

type passkeyCreateMessage struct {
	passkeyOrigin

	Options *passkey.CreationOptions `json:"options"`
}

type passkeyQueryMessage struct {
	passkeyOrigin

	Options *passkey.RequestOptions `json:"options"`
}

type passkeyGetMessage struct {
	passkeyOrigin

	Options      *passkey.RequestOptions `json:"options"`
	CredentialID passkey.Base64URL       `json:"credential_id"`
}

type passkeyCreateResponse struct {
	Entry      string                        `json:"entry"`
	Credential *passkey.RegistrationResponse `json:"credential"`
	// Warning is set if the passkey was stored, but the store reported a
	// failure afterwards, e.g. because it could not push to its remote.
	Warning string `json:"warning,omitempty"`
}

type passkeyCandidate struct {
	ID              passkey.Base64URL `json:"id"`
	Entry           string            `json:"entry"`
	UserName        string            `json:"user_name"`
	UserDisplayName string            `json:"user_display_name"`
}

type passkeyQueryResponse struct {
	Credentials []passkeyCandidate `json:"credentials"`
}

type passkeyGetResponse struct {
	Entry      string                          `json:"entry"`
	Credential *passkey.AuthenticationResponse `json:"credential"`
	// Warning is set if the signature counter of the passkey was stored, but
	// the store reported a failure afterwards.
	Warning string `json:"warning,omitempty"`
}

// passkeyErrorResponse is the response to a failed WebAuthn ceremony.
type passkeyErrorResponse struct {
	// Error is the reason of the failure. It is meant for the user and must
	// not be passed on to the web page.
	Error string `json:"error"`
	// ErrorName is the name of the DOMException to reject the promise of the
	// WebAuthn API with.
	ErrorName string `json:"error_name"`
	// Fallback is true if the request should be handed to another
	// authenticator: gopass has no matching passkey or is not suitable.
	Fallback bool `json:"fallback"`
}

// errorMessage returns the response to send for an error.
func errorMessage(err error) any {
	var pe *passkey.Error
	if errors.As(err, &pe) {
		return passkeyErrorResponse{
			Error:     err.Error(),
			ErrorName: string(pe.Name),
			Fallback:  passkey.IsFallback(err),
		}
	}

	return errorResponse{
		Error: err.Error(),
	}
}

// warning returns the warning of a ceremony that succeeded.
func warning(entry *passkey.Entry) string {
	if entry.Warning == nil {
		return ""
	}

	return entry.Warning.Error()
}

// authenticator returns the WebAuthn authenticator backed by the store.
func (api *API) authenticator(ctx context.Context) *passkey.Authenticator {
	var consent passkey.ConsentFunc
	if api.Confirm != nil {
		consent = passkey.ConsentByConfirmation(api.Confirm)
	}

	return passkey.NewAuthenticator(passkey.NewStore(ctx, newNativePasskeyStore(api.Store)), consent)
}

const passkeyWriteTimeout = 2 * time.Minute

// nativePasskeyStore keeps an already started write alive when the browser
// disconnects. Set writes the encrypted file and commits it to Git inline
// in the native host. Cancelling partway through could leave it uncommitted.
type nativePasskeyStore struct {
	passkey.SecretStore
}

// passkeyStoreScope is the optional part of passkey.SecretStore used by
// passkey.NewStore for recipient scope and path alias checks.
type passkeyStoreScope interface {
	SameRecipientScope(ctx context.Context, a, b string) bool
	Exists(ctx context.Context, name string) bool
}

func newNativePasskeyStore(store passkey.SecretStore) passkey.SecretStore {
	// Preserve the optional checks without claiming support on stores that
	// do not implement them. Both wrappers use the same Set deadline.
	native := nativePasskeyStore{store}
	if scope, ok := store.(passkeyStoreScope); ok {
		return struct {
			nativePasskeyStore
			passkeyStoreScope
		}{native, scope}
	}

	return native
}

func (s nativePasskeyStore) Set(ctx context.Context, name string, sec gopass.Byter) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("not starting passkey write: %w", err)
	}

	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), passkeyWriteTimeout)
	defer cancel()

	return s.SecretStore.Set(writeCtx, name, sec) //nolint:wrapcheck
}

// ErrDisconnected is returned when a ceremony ends because the browser
// closed the connection.
var ErrDisconnected = errors.New("the browser closed the connection")

// untilDisconnected returns a context that is cancelled when the input ends,
// i.e. when the browser closes the connection.
//
// A browser keeps the input of a native messaging host open until it has the
// response. It closes it when nobody waits for the response anymore: when
// the extension disconnects the port it sent the request through, e.g.
// because the web page aborted the call, or when the browser shuts the host
// down. A ceremony that is waiting for the user has to end and take its
// dialog down. With redirected regular-file input, EOF is expected and
// means nothing about whether the output is still being read.
func (api *API) untilDisconnected(ctx context.Context) (context.Context, context.CancelFunc) {
	if file, ok := api.Reader.(*os.File); ok {
		if info, err := file.Stat(); err == nil && info.Mode().IsRegular() {
			return context.WithCancel(ctx)
		}
	}

	ctx, cancel := context.WithCancelCause(ctx)

	go func() {
		// The host answers a single request. Anything that arrives after
		// it is not read by anyone and can be dropped.
		_, _ = io.Copy(io.Discard, api.Reader)
		cancel(ErrDisconnected)
	}()

	return ctx, func() { cancel(nil) }
}

// ceremonyError preserves the disconnect cause even if the authenticator
// reports only context.Canceled or returned successfully just before EOF.
func ceremonyError(ctx context.Context, err error) error {
	if errors.Is(context.Cause(ctx), ErrDisconnected) {
		return ErrDisconnected
	}

	return err
}

// respondPasskeyCreate performs a registration ceremony: it creates and
// stores a new passkey.
func (api *API) respondPasskeyCreate(ctx context.Context, msgBytes []byte) error {
	var message passkeyCreateMessage
	if err := json.Unmarshal(msgBytes, &message); err != nil {
		return &passkey.Error{Name: passkey.TypeError, Err: fmt.Errorf("failed to unmarshal JSON message: %w", err)}
	}

	ctx, cancel := api.untilDisconnected(ctx)
	defer cancel()

	rsp, entry, err := api.authenticator(ctx).Register(ctx, message.origin(), message.Options)
	if err := ceremonyError(ctx, err); err != nil {
		return err
	}

	return sendResponse(passkeyCreateResponse{
		Entry:      entry.Name,
		Credential: rsp,
		Warning:    warning(entry),
	}, api.Writer)
}

// respondPasskeyQuery lists the passkeys that can be used to answer an
// authentication request, so that the user can pick one.
func (api *API) respondPasskeyQuery(ctx context.Context, msgBytes []byte) error {
	var message passkeyQueryMessage
	if err := json.Unmarshal(msgBytes, &message); err != nil {
		return &passkey.Error{Name: passkey.TypeError, Err: fmt.Errorf("failed to unmarshal JSON message: %w", err)}
	}

	// The request is checked in the same order as passkeyGet does, so that
	// both fail the same way.
	entries, err := api.authenticator(ctx).Candidates(ctx, message.origin(), message.Options)
	if err != nil {
		return err
	}

	// Do not let the user pick a passkey for a request that passkeyGet is
	// going to refuse. Every ceremony is subject to the rule of
	// passkey.ConsentByConfirmation (see authenticator): the user is asked
	// to confirm, which is not a verification.
	if message.Options.UserVerification == "required" && !passkey.UserVerificationEnabled(ctx) {
		return &passkey.Error{
			Name: passkey.NotAllowedError,
			Err: fmt.Errorf("%w: the site requires user verification, which gopass only reports if passkey.user-verification is enabled",
				passkey.ErrUnsuitable),
		}
	}

	candidates := make([]passkeyCandidate, 0, len(entries))
	for _, e := range entries {
		candidates = append(candidates, passkeyCandidate{
			ID:              e.Credential.ID,
			Entry:           e.Name,
			UserName:        e.Credential.UserName,
			UserDisplayName: e.Credential.UserDisplayName,
		})
	}

	return sendResponse(passkeyQueryResponse{
		Credentials: candidates,
	}, api.Writer)
}

// respondPasskeyGet performs an authentication ceremony: it signs the
// challenge of a relying party with a stored passkey.
func (api *API) respondPasskeyGet(ctx context.Context, msgBytes []byte) error {
	var message passkeyGetMessage
	if err := json.Unmarshal(msgBytes, &message); err != nil {
		return &passkey.Error{Name: passkey.TypeError, Err: fmt.Errorf("failed to unmarshal JSON message: %w", err)}
	}

	ctx, cancel := api.untilDisconnected(ctx)
	defer cancel()

	rsp, entry, err := api.authenticator(ctx).Authenticate(ctx, message.origin(), message.Options, message.CredentialID)
	if err := ceremonyError(ctx, err); err != nil {
		return err
	}

	return sendResponse(passkeyGetResponse{
		Entry:      entry.Name,
		Credential: rsp,
		Warning:    warning(entry),
	}, api.Writer)
}
