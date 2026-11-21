package jsonapi

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blang/semver"
	"github.com/gopasspw/gopass/pkg/gopass"
	"github.com/gopasspw/gopass/pkg/gopass/apimock"
	"github.com/gopasspw/gopass/pkg/passkey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twpayne/go-pinentry/v4"
)

// passkeyTester sends messages to an API with an in-memory store.
type passkeyTester struct {
	t     *testing.T
	store gopass.Store
	// confirm is the answer of the user. asked records the questions.
	confirm bool
	asked   []passkey.ConsentRequest
	// pinentry makes the API ask with newPinentry instead.
	pinentry bool
}

func newPasskeyTester(t *testing.T) *passkeyTester {
	t.Helper()

	// do not pick up the config of whoever runs the tests
	t.Setenv("GOPASS_HOMEDIR", t.TempDir())

	return &passkeyTester{
		t:       t,
		store:   apimock.New(),
		confirm: true,
	}
}

// send serves a single message. It returns the response, which is the error
// response if serving the message failed, just like the listen command does.
func (pt *passkeyTester) send(msg string) map[string]any {
	pt.t.Helper()

	var rsp map[string]any
	require.NoError(pt.t, json.Unmarshal([]byte(pt.sendRaw(msg)), &rsp))

	return rsp
}

// connect returns the input of a host that a browser sent msg to. Like the
// input from a browser that waits for the response, it does not end until
// the browser hangs up.
func connect(msg string) (io.Reader, func()) {
	r, w := io.Pipe()

	go func() {
		_, _ = w.Write([]byte(writeMessageWithLength(msg)))
	}()

	return r, func() { _ = w.Close() }
}

// sendRaw is like send, but returns the response as it was sent.
func (pt *passkeyTester) sendRaw(msg string) string {
	pt.t.Helper()

	in, hangUp := connect(msg)
	defer hangUp()

	var out bytes.Buffer

	api := New(pt.store, in, &out, semver.MustParse("1.2.3"))
	if !pt.pinentry {
		api.Confirm = func(_ context.Context, req passkey.ConsentRequest) (bool, error) {
			pt.asked = append(pt.asked, req)

			return pt.confirm, nil
		}
	}

	if err := api.ServeMessage(pt.t.Context()); err != nil {
		require.Empty(pt.t, out.Bytes(), "no response must be sent before an error")
		require.NoError(pt.t, api.SendErrorResponse(err))
	}

	return readAndVerifyMessageLength(pt.t, out.Bytes())
}

// field returns the member of a decoded JSON object at the given path.
func field(t *testing.T, v any, path ...string) any {
	t.Helper()

	for _, name := range path {
		obj, ok := v.(map[string]any)
		require.True(t, ok, "%v has no member %s", v, name)

		v = obj[name]
	}

	return v
}

func unb64(t *testing.T, v any) []byte {
	t.Helper()

	s, ok := v.(string)
	require.True(t, ok, "%v is not a string", v)

	buf, err := base64.RawURLEncoding.DecodeString(s)
	require.NoError(t, err)

	return buf
}

const passkeyCreate = `{
  "type": "passkeyCreate",
  "origin": "https://login.example.com",
  "options": {
    "rp": {"id": "example.com", "name": "Example Inc."},
    "user": {"id": "aGFuZGxlLWFsaWNl", "name": "alice", "displayName": "Alice"},
    "challenge": "cmVnaXN0ZXI",
    "pubKeyCredParams": [{"type": "public-key", "alg": -7}, {"type": "public-key", "alg": -257}],
    "timeout": 60000,
    "excludeCredentials": [],
    "authenticatorSelection": {"residentKey": "required", "userVerification": "preferred"},
    "attestation": "none",
    "extensions": {"credProps": true}
  }
}`

const passkeyGetOptions = `{"challenge": "c2lnbiBpbg", "rpId": "example.com", "allowCredentials": [], "userVerification": "preferred"}`

func TestPasskeyCeremonies(t *testing.T) {
	pt := newPasskeyTester(t)

	// nothing to sign in with yet
	rsp := pt.send(`{"type": "passkeyQuery", "origin": "https://login.example.com", "options": ` + passkeyGetOptions + `}`)
	assert.Equal(t, "NotAllowedError", rsp["error_name"])
	assert.Equal(t, true, rsp["fallback"])
	assert.Contains(t, rsp["error"], "no matching passkey found")
	assert.Empty(t, pt.asked)

	// registration
	rsp = pt.send(passkeyCreate)
	require.NotContains(t, rsp, "error")
	assert.Equal(t, "passkeys/example.com/alice", rsp["entry"])

	require.Len(t, pt.asked, 1)
	assert.Equal(t, passkey.ConsentRequest{
		Operation:       passkey.OperationRegister,
		RPID:            "example.com",
		RPName:          "Example Inc.",
		Origin:          "https://login.example.com",
		UserName:        "alice",
		UserDisplayName: "Alice",
	}, pt.asked[0])

	reg, ok := rsp["credential"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "public-key", reg["type"])
	assert.Equal(t, "platform", reg["authenticatorAttachment"])
	assert.Equal(t, reg["id"], reg["rawId"])
	assert.Equal(t, map[string]any{"credProps": map[string]any{"rk": true}}, reg["clientExtensionResults"])

	regRsp, ok := reg["response"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []any{"internal"}, regRsp["transports"])
	assert.InDelta(t, -7, regRsp["publicKeyAlgorithm"], 0)
	assert.NotEmpty(t, regRsp["attestationObject"])
	assert.JSONEq(t,
		`{"type":"webauthn.create","challenge":"cmVnaXN0ZXI","origin":"https://login.example.com","crossOrigin":false}`,
		string(unb64(t, regRsp["clientDataJSON"])))

	// The passkey is stored as a single line of a secret.
	sec, err := pt.store.Get(t.Context(), "passkeys/example.com/alice", "latest")
	require.NoError(t, err)

	line, found := sec.Get(passkey.SecretKey)
	require.True(t, found)
	assert.True(t, strings.HasPrefix(line, `{"v":1,"rpId":"example.com","userName":"alice","userDisplayName":"Alice",`), line)

	var stored map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &stored))

	// what the user can choose from
	query := `{"type": "passkeyQuery", "origin": "https://login.example.com", "options": ` + passkeyGetOptions + `}`
	rsp = pt.send(query)
	assert.Equal(t, map[string]any{
		"credentials": []any{
			map[string]any{
				"id":                reg["id"],
				"entry":             "passkeys/example.com/alice",
				"user_name":         "alice",
				"user_display_name": "Alice",
			},
		},
	}, rsp)
	assert.Len(t, pt.asked, 1, "listing the passkeys does not need consent")

	// authentication
	rsp = pt.send(`{"type": "passkeyGet", "origin": "https://login.example.com", "options": ` + passkeyGetOptions + `}`)
	require.NotContains(t, rsp, "error")
	assert.Equal(t, "passkeys/example.com/alice", rsp["entry"])

	require.Len(t, pt.asked, 2)
	assert.Equal(t, passkey.ConsentRequest{
		Operation:       passkey.OperationAuthenticate,
		RPID:            "example.com",
		Origin:          "https://login.example.com",
		UserName:        "alice",
		UserDisplayName: "Alice",
		Entry:           "passkeys/example.com/alice",
	}, pt.asked[1])

	auth, ok := rsp["credential"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, reg["id"], auth["id"])
	assert.Equal(t, "public-key", auth["type"])
	assert.Equal(t, map[string]any{}, auth["clientExtensionResults"])

	authRsp, ok := auth["response"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "aGFuZGxlLWFsaWNl", authRsp["userHandle"])

	clientData := unb64(t, authRsp["clientDataJSON"])
	assert.JSONEq(t,
		`{"type":"webauthn.get","challenge":"c2lnbiBpbg","origin":"https://login.example.com","crossOrigin":false}`,
		string(clientData))

	// The relying party verifies the assertion with the public key it got
	// at the registration.
	pub, err := x509.ParsePKIXPublicKey(unb64(t, regRsp["publicKey"]))
	require.NoError(t, err)

	key, ok := pub.(*ecdsa.PublicKey)
	require.True(t, ok)

	authData := unb64(t, authRsp["authenticatorData"])
	rpIDHash := sha256.Sum256([]byte("example.com"))
	require.Len(t, authData, 37)
	assert.Equal(t, rpIDHash[:], authData[:32])
	assert.Equal(t, byte(0x19), authData[32], "user present, backup eligible and backed up")

	hash := sha256.Sum256(clientData)
	digest := sha256.Sum256(append(authData, hash[:]...))
	assert.True(t, ecdsa.VerifyASN1(key, digest[:], unb64(t, authRsp["signature"])))

	// No response ever contains the private key.
	privateKey, ok := stored["key"].(string)
	require.True(t, ok)
	require.NotEmpty(t, privateKey)

	for _, msg := range []string{
		passkeyCreate,
		query,
		`{"type": "passkeyGet", "origin": "https://login.example.com", "options": ` + passkeyGetOptions + `}`,
	} {
		out, err := json.Marshal(pt.send(msg))
		require.NoError(t, err)
		assert.NotContains(t, string(out), privateKey[:24])
	}
}

func TestPasskeySelection(t *testing.T) {
	pt := newPasskeyTester(t)

	alice := pt.send(passkeyCreate)
	require.NotContains(t, alice, "error")

	bob := pt.send(strings.NewReplacer("aGFuZGxlLWFsaWNl", "aGFuZGxlLWJvYg", `"alice"`, `"bob"`, `"Alice"`, `"Bob"`).Replace(passkeyCreate))
	require.NotContains(t, bob, "error")
	assert.Equal(t, "passkeys/example.com/bob", bob["entry"])

	bobID, ok := field(t, bob, "credential", "id").(string)
	require.True(t, ok)

	rsp := pt.send(`{"type": "passkeyQuery", "origin": "https://example.com", "options": ` + passkeyGetOptions + `}`)
	assert.Len(t, rsp["credentials"], 2)

	// With more than one passkey the user has to choose first.
	asked := len(pt.asked)
	rsp = pt.send(`{"type": "passkeyGet", "origin": "https://example.com", "options": ` + passkeyGetOptions + `}`)
	assert.Equal(t, "NotAllowedError", rsp["error_name"])
	assert.Equal(t, false, rsp["fallback"])
	assert.Contains(t, rsp["error"], "selection is required")
	assert.Len(t, pt.asked, asked)

	rsp = pt.send(`{"type": "passkeyGet", "origin": "https://example.com", "credential_id": "` + bobID + `", "options": ` + passkeyGetOptions + `}`)
	require.NotContains(t, rsp, "error")
	assert.Equal(t, "passkeys/example.com/bob", rsp["entry"])
	assert.Equal(t, bobID, field(t, rsp, "credential", "id"))
	assert.Equal(t, "bob", pt.asked[len(pt.asked)-1].UserName)

	// The relying party can restrict the passkeys it accepts.
	allow := strings.Replace(passkeyGetOptions, `"allowCredentials": []`, `"allowCredentials": [{"type": "public-key", "id": "`+bobID+`"}]`, 1)
	rsp = pt.send(`{"type": "passkeyQuery", "origin": "https://example.com", "options": ` + allow + `}`)
	require.Len(t, rsp["credentials"], 1)

	rsp = pt.send(`{"type": "passkeyGet", "origin": "https://example.com", "options": ` + allow + `}`)
	require.NotContains(t, rsp, "error")
	assert.Equal(t, bobID, field(t, rsp, "credential", "id"))

	// A site is told about a passkey it wants to exclude only if the user
	// agrees. Either way no passkey is created.
	exclude := strings.Replace(passkeyCreate, `"excludeCredentials": []`, `"excludeCredentials": [{"type": "public-key", "id": "`+bobID+`"}]`, 1)
	rsp = pt.send(exclude)
	assert.Equal(t, "InvalidStateError", rsp["error_name"])
	assert.Equal(t, false, rsp["fallback"])
	assert.Equal(t, passkey.OperationDisclose, pt.asked[len(pt.asked)-1].Operation)

	pt.confirm = false
	rsp = pt.send(exclude)
	assert.Equal(t, "NotAllowedError", rsp["error_name"])

	names, err := pt.store.List(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{"passkeys/example.com/alice", "passkeys/example.com/bob"}, names)
}

func TestPasskeyErrors(t *testing.T) {
	pt := newPasskeyTester(t)

	rsp := pt.send(passkeyCreate)
	require.NotContains(t, rsp, "error")

	get := func(origin, options string) string {
		return `{"type": "passkeyGet", "origin": "` + origin + `", "options": ` + options + `}`
	}
	query := func(origin, options string) string {
		return strings.Replace(get(origin, options), "passkeyGet", "passkeyQuery", 1)
	}
	required := strings.Replace(passkeyGetOptions, "preferred", "required", 1)

	for name, tc := range map[string]struct {
		msg      string
		name     string
		fallback bool
	}{
		// The passkey of example.com is not available to any other site.
		"other site": {get("https://evil.example", passkeyGetOptions), "SecurityError", false},
		"lookalike":  {get("https://example.com.evil.example", passkeyGetOptions), "SecurityError", false},
		"insecure":   {get("http://example.com", passkeyGetOptions), "SecurityError", false},
		"other site, default rp id": {
			get("https://evil.example", `{"challenge": "c2lnbiBpbg"}`), "NotAllowedError", true,
		},
		"public suffix": {
			get("https://example.com", `{"challenge": "c2lnbiBpbg", "rpId": "com"}`), "SecurityError", false,
		},
		"no origin":     {`{"type": "passkeyGet", "options": ` + passkeyGetOptions + `}`, "NotAllowedError", false},
		"opaque origin": {get("null", passkeyGetOptions), "NotAllowedError", false},
		"cross origin without top origin": {
			`{"type": "passkeyGet", "origin": "https://example.com", "cross_origin": true, "options": ` + passkeyGetOptions + `}`,
			"NotAllowedError", false,
		},
		"no options":       {`{"type": "passkeyGet", "origin": "https://example.com"}`, "TypeError", false},
		"no challenge":     {get("https://example.com", `{"rpId": "example.com"}`), "TypeError", false},
		"malformed":        {get("https://example.com", `{"challenge": 42}`), "TypeError", false},
		"not base64url":    {get("https://example.com", `{"challenge": "+/+/"}`), "TypeError", false},
		"no such passkey":  {get("https://example.org", `{"challenge": "c2lnbiBpbg"}`), "NotAllowedError", true},
		"query other site": {query("https://evil.example", passkeyGetOptions), "SecurityError", false},
		// The origin is checked before anything else, by both requests.
		"other site, verification required":       {get("https://evil.example", required), "SecurityError", false},
		"query other site, verification required": {query("https://evil.example", required), "SecurityError", false},
		"no such passkey, verification required": {
			get("https://example.org", `{"challenge": "c2lnbiBpbg", "userVerification": "required"}`), "NotAllowedError", true,
		},
		"query no such passkey, verification required": {
			query("https://example.org", `{"challenge": "c2lnbiBpbg", "userVerification": "required"}`), "NotAllowedError", true,
		},
		"query malformed": {`{"type": "passkeyQuery", "options": []}`, "TypeError", false},
		"create other site": {
			strings.Replace(passkeyCreate, "https://login.example.com", "https://evil.example", 1), "SecurityError", false,
		},
		"create malformed": {`{"type": "passkeyCreate", "options": "none"}`, "TypeError", false},
		// What gopass does not support, another authenticator may.
		"create without algorithm": {
			strings.Replace(passkeyCreate, `{"type": "public-key", "alg": -7}, {"type": "public-key", "alg": -257}`, `{"type": "public-key", "alg": -36}`, 1),
			"NotSupportedError", true,
		},
		// A site that wants a security key is left to the browser.
		"create roaming": {
			strings.Replace(passkeyCreate, `"residentKey": "required"`, `"authenticatorAttachment": "cross-platform"`, 1),
			"NotAllowedError", true,
		},
		// A confirmation is no verification of the user.
		"verification required":       {get("https://example.com", required), "NotAllowedError", true},
		"query verification required": {query("https://example.com", required), "NotAllowedError", true},
	} {
		asked := len(pt.asked)

		rsp := pt.send(tc.msg)
		assert.Equal(t, tc.name, rsp["error_name"], "%s: %v", name, rsp)
		assert.Equal(t, tc.fallback, rsp["fallback"], "%s: %v", name, rsp)
		assert.NotEmpty(t, rsp["error"], name)
		assert.NotContains(t, rsp, "credential", name)

		assert.Len(t, pt.asked, asked, "%s: the user must not be bothered", name)
	}

	// declined by the user
	pt.confirm = false

	rsp = pt.send(get("https://example.com", passkeyGetOptions))
	assert.Equal(t, "NotAllowedError", rsp["error_name"])
	assert.Equal(t, false, rsp["fallback"])
	assert.NotContains(t, rsp, "credential")

	rsp = pt.send(strings.NewReplacer("aGFuZGxlLWFsaWNl", "aGFuZGxlLWJvYg", `"alice"`, `"bob"`).Replace(passkeyCreate))
	assert.Equal(t, "NotAllowedError", rsp["error_name"])
	assert.Equal(t, false, rsp["fallback"])

	names, err := pt.store.List(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{"passkeys/example.com/alice"}, names)

	// Errors that have nothing to do with passkeys look like they always did.
	rsp = pt.send(`{"type": "getLogin", "entry": "missing"}`)
	assert.Contains(t, rsp["error"], "failed to get secret")
	assert.NotContains(t, rsp, "error_name")
	assert.NotContains(t, rsp, "fallback")
}

func TestPasskeyUserVerification(t *testing.T) {
	pt := newPasskeyTester(t)

	require.NotContains(t, pt.send(passkeyCreate), "error")

	required := `{"type": "passkeyGet", "origin": "https://example.com", "options": ` +
		strings.Replace(passkeyGetOptions, "preferred", "required", 1) + `}`

	// A confirmation is no verification of the user. The user is not even
	// asked for one that does not help, or to pick a passkey.
	for _, msg := range []string{required, strings.Replace(required, "passkeyGet", "passkeyQuery", 1)} {
		rsp := pt.send(msg)
		assert.Equal(t, "NotAllowedError", rsp["error_name"])
		assert.Equal(t, true, rsp["fallback"])
		assert.Contains(t, rsp["error"], "passkey.user-verification")
		assert.NotContains(t, rsp, "credentials")
		assert.Len(t, pt.asked, 1)
	}

	// The user declared that using the store amounts to a verification.
	cfgDir := filepath.Join(os.Getenv("GOPASS_HOMEDIR"), ".config", "gopass")
	require.NoError(t, os.MkdirAll(cfgDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "config"), []byte("[passkey]\n\tuser-verification = true\n"), 0o600))

	rsp := pt.send(strings.Replace(required, "passkeyGet", "passkeyQuery", 1))
	assert.Len(t, rsp["credentials"], 1)

	rsp = pt.send(required)
	require.NotContains(t, rsp, "error")

	authData := unb64(t, field(t, rsp, "credential", "response", "authenticatorData"))
	assert.Equal(t, byte(0x1d), authData[32], "user present and verified")
	assert.Equal(t, passkey.OperationAuthenticate, pt.asked[len(pt.asked)-1].Operation)
	assert.True(t, pt.asked[len(pt.asked)-1].VerificationRequired)
}

func TestPasskeyIsNotExposedByOtherRequests(t *testing.T) {
	pt := newPasskeyTester(t)

	require.NotContains(t, pt.send(passkeyCreate), "error")

	// The secret holds more than the passkey.
	sec, err := pt.store.Get(t.Context(), "passkeys/example.com/alice", "latest")
	require.NoError(t, err)
	sec.SetPassword("hunter2")
	require.NoError(t, sec.Set("url", "https://example.com"))
	require.NoError(t, pt.store.Set(t.Context(), "passkeys/example.com/alice", sec))

	line, found := sec.Get(passkey.SecretKey)
	require.True(t, found)

	var stored map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &stored))

	privateKey, ok := stored["key"].(string)
	require.True(t, ok)
	require.NotEmpty(t, privateKey)

	asked := len(pt.asked)

	// The requests that hand out the content of a secret hand out
	// everything but the passkey. A passkey is only used with the consent
	// of the user, and its private key never leaves the store.
	rsp := pt.send(`{"type": "getData", "entry": "passkeys/example.com/alice"}`)
	assert.Equal(t, "https://example.com", rsp["url"])
	assert.NotContains(t, rsp, passkey.SecretKey)

	rsp = pt.send(`{"type": "copyToClipboard", "entry": "passkeys/example.com/alice", "key": "passkey"}`)
	assert.Contains(t, rsp["error"], "passkeys can not be copied")

	rsp = pt.send(`{"type": "getLogin", "entry": "passkeys/example.com/alice"}`)
	assert.Equal(t, map[string]any{"username": "alice", "password": "hunter2"}, rsp)

	for _, msg := range []string{
		`{"type": "getData", "entry": "passkeys/example.com/alice"}`,
		`{"type": "getLogin", "entry": "passkeys/example.com/alice"}`,
		`{"type": "copyToClipboard", "entry": "passkeys/example.com/alice", "key": "passkey"}`,
		`{"type": "query", "query": "example.com"}`,
		`{"type": "queryHost", "host": "example.com"}`,
	} {
		out := pt.sendRaw(msg)
		assert.NotContains(t, out, privateKey[:24], msg)
		assert.NotContains(t, out, "credentialId", msg)
	}

	assert.Len(t, pt.asked, asked)
}

func TestPasskeyLargeRequest(t *testing.T) {
	pt := newPasskeyTester(t)

	require.NotContains(t, pt.send(passkeyCreate), "error")

	// A relying party with many passkeys per account sends long lists.
	// Such a request does not arrive in one piece.
	allow := make([]string, 0, 200)
	for i := range 200 {
		allow = append(allow, fmt.Sprintf(`{"type": "public-key", "id": "%s", "transports": ["internal", "hybrid"]}`,
			base64.RawURLEncoding.EncodeToString(fmt.Appendf(nil, "an unknown credential with the number %04d", i))))
	}

	msg := `{"type": "passkeyQuery", "origin": "https://example.com", "options": {"challenge": "c2lnbiBpbg", "rpId": "example.com", "allowCredentials": [` +
		strings.Join(allow, ",") + `]}}`
	require.Greater(t, len(msg), 16*1024)

	var out bytes.Buffer

	api := New(pt.store, &trickleReader{data: []byte(writeMessageWithLength(msg))}, &out, semver.MustParse("1.2.3"))

	// It is read completely: the answer is about its content.
	err := api.ServeMessage(t.Context())
	require.ErrorIs(t, err, passkey.ErrNoCredentials)
}

// unsyncedStore is a store that saves a secret and then reports a failure,
// like a store whose remote can not be reached.
type unsyncedStore struct {
	gopass.Store
}

func (u unsyncedStore) Set(ctx context.Context, name string, sec gopass.Byter) error {
	if err := u.Store.Set(ctx, name, sec); err != nil {
		return err
	}

	return errors.New("failed to push to git remote")
}

func TestPasskeyUnsyncedStore(t *testing.T) {
	pt := newPasskeyTester(t)
	pt.store = unsyncedStore{pt.store}

	// The passkey is stored, so the registration succeeds. That the store
	// could not sync is no reason to leave the site without the passkey.
	rsp := pt.send(passkeyCreate)
	require.NotContains(t, rsp, "error")
	assert.Equal(t, "passkeys/example.com/alice", rsp["entry"])
	assert.Contains(t, rsp["warning"], "failed to push to git remote")
	assert.Contains(t, rsp, "credential")

	sec, err := pt.store.Get(t.Context(), "passkeys/example.com/alice", "latest")
	require.NoError(t, err)

	creds, err := passkey.FromSecret(sec)
	require.NoError(t, err)
	require.Len(t, creds, 1)
	assert.Equal(t, field(t, rsp, "credential", "id"), passkey.Base64URL(creds[0].ID).String())

	// Signing in writes nothing, so there is nothing to warn about.
	rsp = pt.send(`{"type": "passkeyGet", "origin": "https://example.com", "options": ` + passkeyGetOptions + `}`)
	require.NotContains(t, rsp, "error")
	assert.NotContains(t, rsp, "warning")
}

func TestPasskeyNeedsConsent(t *testing.T) {
	t.Setenv("GOPASS_HOMEDIR", t.TempDir())

	// The API asks with a pinentry unless told otherwise.
	api := New(apimock.New(), &bytes.Buffer{}, &bytes.Buffer{}, semver.MustParse("1.2.3"))
	require.NotNil(t, api.Confirm)

	// Without a way to ask the user no passkey is created.
	in, hangUp := connect(passkeyCreate)
	defer hangUp()

	api = &API{Store: apimock.New(), Reader: in, Writer: &bytes.Buffer{}}

	err := api.ServeMessage(t.Context())
	require.ErrorIs(t, err, passkey.ErrDeclined)

	names, err := api.Store.List(t.Context())
	require.NoError(t, err)
	assert.Empty(t, names)
}

func TestPasskeyDisconnect(t *testing.T) {
	pt := newPasskeyTester(t)
	require.NotContains(t, pt.send(passkeyCreate), "error")

	for _, msg := range []string{
		strings.NewReplacer("aGFuZGxlLWFsaWNl", "aGFuZGxlLWJvYg", `"alice"`, `"bob"`).Replace(passkeyCreate),
		`{"type": "passkeyGet", "origin": "https://example.com", "options": ` + passkeyGetOptions + `}`,
	} {
		in, hangUp := connect(msg)
		defer hangUp()

		var out bytes.Buffer

		api := New(pt.store, in, &out, semver.MustParse("1.2.3"))

		asked := make(chan struct{})
		api.Confirm = func(ctx context.Context, _ passkey.ConsentRequest) (bool, error) {
			close(asked)
			// The user does not answer before the dialog is taken down.
			<-ctx.Done()

			return false, context.Cause(ctx)
		}

		// The browser gives up on the request while the user is asked,
		// e.g. because the extension disconnected the port.
		go func() {
			<-asked
			hangUp()
		}()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		err := api.ServeMessage(ctx)
		cancel()

		require.ErrorIs(t, err, ErrDisconnected, msg)
		assert.False(t, passkey.IsFallback(err), msg)
		assert.Empty(t, out.Bytes(), "nobody is there to receive a response")
	}

	names, err := pt.store.List(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{"passkeys/example.com/alice"}, names)
}

func TestPasskeyFromRegularFile(t *testing.T) {
	pt := newPasskeyTester(t)
	path := filepath.Join(t.TempDir(), "request.bin")
	require.NoError(t, os.WriteFile(path, []byte(writeMessageWithLength(passkeyCreate)), 0o600))
	in, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, in.Close()) })

	var out bytes.Buffer
	api := New(pt.store, in, &out, semver.MustParse("1.2.3"))
	api.Confirm = func(ctx context.Context, _ passkey.ConsentRequest) (bool, error) {
		return true, ctx.Err()
	}

	require.NoError(t, api.ServeMessage(t.Context()))
	assert.Contains(t, readAndVerifyMessageLength(t, out.Bytes()), `"entry":"passkeys/example.com/alice"`)
}

func TestQueryHostOmitsPasskeyEntries(t *testing.T) {
	pt := newPasskeyTester(t)
	cfgDir := filepath.Join(os.Getenv("GOPASS_HOMEDIR"), ".config", "gopass")
	require.NoError(t, os.MkdirAll(cfgDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "config"), []byte("[passkey]\n\tprefix = custom/keys\n"), 0o600))
	require.NotContains(t, pt.send(passkeyCreate), "error")

	for _, name := range []string{"passkeys/login.example.com/bob", "team/Passkeys/login.example.com/shared", "websites/example.com/password"} {
		require.NoError(t, pt.store.Set(t.Context(), name, newSec(t, "password\n")))
	}

	// Passkeys at the exact host do not prevent password matches at the parent.
	assert.JSONEq(t, `["websites/example.com/password"]`, pt.sendRaw(`{"type":"queryHost","host":"login.example.com"}`))
	assert.JSONEq(t, `["websites/example.com/password"]`, pt.sendRaw(`{"type":"queryHost","host":"example.com"}`))
	assert.Contains(t, pt.sendRaw(`{"type":"query","query":"example.com"}`), "custom/keys/example.com/alice")
}

type storeWithSetHook struct {
	gopass.Store
	beforeSet func(context.Context)
}

func (s storeWithSetHook) Set(ctx context.Context, name string, sec gopass.Byter) error {
	s.beforeSet(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}

	return s.Store.Set(ctx, name, sec)
}

func TestPasskeyDisconnectDuringWrite(t *testing.T) {
	for _, operation := range []string{"register", "advance counter"} {
		t.Run(operation, func(t *testing.T) {
			pt := newPasskeyTester(t)
			t.Setenv("GOPASS_CONFIG_NOSYSTEM", "true")
			cfgDir := filepath.Join(os.Getenv("GOPASS_HOMEDIR"), ".config", "gopass")
			require.NoError(t, os.MkdirAll(cfgDir, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "config"), []byte("[passkey]\n\tsigncount = true\n"), 0o600))

			msg := passkeyCreate
			wantCount := uint32(1)
			if operation == "advance counter" {
				require.NotContains(t, pt.send(passkeyCreate), "error")
				msg = `{"type": "passkeyGet", "origin": "https://example.com", "options": ` + passkeyGetOptions + `}`
				wantCount++
			}

			in, hangUp := connect(msg)
			defer hangUp()

			var (
				out         bytes.Buffer
				approvalCtx context.Context
			)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			store := storeWithSetHook{pt.store, func(writeCtx context.Context) {
				hangUp()
				<-approvalCtx.Done()
				require.ErrorIs(t, context.Cause(approvalCtx), ErrDisconnected)
				require.NoError(t, writeCtx.Err())
				deadline, bounded := writeCtx.Deadline()
				assert.True(t, bounded)
				assert.Greater(t, time.Until(deadline), 5*time.Second)
				assert.LessOrEqual(t, time.Until(deadline), passkeyWriteTimeout)
			}}
			api := New(store, in, &out, semver.MustParse("1.2.3"))
			api.Confirm = func(ctx context.Context, _ passkey.ConsentRequest) (bool, error) {
				approvalCtx = ctx

				return true, nil
			}

			err := api.ServeMessage(ctx)
			require.ErrorIs(t, err, ErrDisconnected)
			assert.Empty(t, out.Bytes())

			sec, err := pt.store.Get(t.Context(), "passkeys/example.com/alice", "latest")
			require.NoError(t, err)
			creds, err := passkey.FromSecret(sec)
			require.NoError(t, err)
			require.Len(t, creds, 1)
			assert.Equal(t, wantCount, creds[0].SignCount)
		})
	}
}

func TestPasskeyWithoutDialog(t *testing.T) {
	pt := newPasskeyTester(t)
	require.NotContains(t, pt.send(passkeyCreate), "error")

	orig := newPinentry
	t.Cleanup(func() { newPinentry = orig })

	pt.pinentry = true

	answering := func(err error) func(context.Context, string) (confirmer, error) {
		return func(context.Context, string) (confirmer, error) {
			return &fakePinentry{err: err}, nil
		}
	}

	for name, tc := range map[string]struct {
		pinentry func(context.Context, string) (confirmer, error)
		fallback bool
	}{
		// The browser can ask instead, so the request is left to it.
		"no pinentry": {
			func(context.Context, string) (confirmer, error) {
				return nil, errors.New(`exec: "pinentry": executable file not found in $PATH`)
			},
			true,
		},
		"no terminal": {
			answering(&pinentry.AssuanError{Code: 83918950, Description: "Inappropriate ioctl for device"}),
			true,
		},
		// The user was asked.
		"declined":  {answering(&pinentry.AssuanError{Code: pinentry.AssuanErrorCodeCancelled}), false},
		"timed out": {answering(&pinentry.AssuanError{Code: assuanTimeout, Description: "Timeout"}), false},
	} {
		newPinentry = tc.pinentry

		for _, msg := range []string{
			strings.NewReplacer("aGFuZGxlLWFsaWNl", "aGFuZGxlLWJvYg", `"alice"`, `"bob"`).Replace(passkeyCreate),
			`{"type": "passkeyGet", "origin": "https://example.com", "options": ` + passkeyGetOptions + `}`,
		} {
			rsp := pt.send(msg)
			assert.Equal(t, "NotAllowedError", rsp["error_name"], "%s: %v", name, rsp)
			assert.Equal(t, tc.fallback, rsp["fallback"], "%s: %v", name, rsp)
			assert.NotContains(t, rsp, "credential", name)
		}
	}

	names, err := pt.store.List(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{"passkeys/example.com/alice"}, names)
}

// fakePinentry is a pinentry that answers without showing a dialog.
type fakePinentry struct {
	confirmed bool
	err       error
	// wait makes Confirm block until the channel is closed.
	wait   <-chan struct{}
	closed bool
}

func (f *fakePinentry) Confirm(string) (bool, error) {
	if f.wait != nil {
		<-f.wait
	}

	return f.confirmed, f.err
}

func (f *fakePinentry) Close() error {
	f.closed = true

	return nil
}

func TestPinentryConfirm(t *testing.T) {
	t.Setenv("GOPASS_HOMEDIR", t.TempDir())

	req := passkey.ConsentRequest{
		Operation: passkey.OperationAuthenticate,
		RPID:      "example.com",
		Origin:    "https://login.example.com",
		UserName:  "alice",
		Entry:     "passkeys/example.com/alice",
	}

	var (
		fake  *fakePinentry
		shown string
	)

	orig := newPinentry
	t.Cleanup(func() { newPinentry = orig })

	newPinentry = func(_ context.Context, desc string) (confirmer, error) {
		shown = desc

		return fake, nil
	}

	// confirmed
	fake = &fakePinentry{confirmed: true}
	consent, err := pinentryConfirm(t.Context(), req)
	require.NoError(t, err)
	assert.True(t, consent)
	assert.Equal(t, "Sign in to example.com as \"alice\"?\n\nPasskey: \"passkeys/example.com/alice\"\nRequested by https://login.example.com", shown)
	assert.True(t, fake.closed)

	// declined, with a button or by closing the dialog
	for _, declined := range []*fakePinentry{
		{confirmed: false},
		{err: &pinentry.AssuanError{Code: pinentry.AssuanErrorCodeCancelled}},
		{err: &pinentry.AssuanError{Code: assuanNotConfirmed}},
	} {
		fake = declined
		consent, err = pinentryConfirm(t.Context(), req)
		require.NoError(t, err)
		assert.False(t, consent)
		assert.True(t, fake.closed)
	}

	// A dialog that failed is no consent, whatever it claims. One that
	// could not be shown leaves the request to another authenticator, one
	// that nobody answered does not.
	for _, tc := range []struct {
		fake       *fakePinentry
		unsuitable bool
	}{
		{&fakePinentry{confirmed: true, err: errors.New("no display")}, true},
		{&fakePinentry{confirmed: true, err: &pinentry.AssuanError{Code: 83918950, Description: "Inappropriate ioctl for device"}}, true},
		{&fakePinentry{confirmed: true, err: &pinentry.AssuanError{Code: assuanTimeout, Description: "Timeout"}}, false},
	} {
		fake = tc.fake
		consent, err = pinentryConfirm(t.Context(), req)
		require.Error(t, err)
		assert.Equal(t, tc.unsuitable, errors.Is(err, passkey.ErrUnsuitable), err)
		assert.False(t, consent)
		assert.True(t, fake.closed)
	}

	// Neither is one that was taken down because the ceremony was cancelled
	// while the user was asked.
	ctx, cancel := context.WithCancel(t.Context())
	fake = &fakePinentry{confirmed: true, wait: ctx.Done()}
	cancel()

	consent, err = pinentryConfirm(ctx, req)
	require.ErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, passkey.ErrUnsuitable)
	assert.False(t, consent)
	assert.True(t, fake.closed)

	// The cause is reported, e.g. that the browser gave up.
	ctx, cancelCause := context.WithCancelCause(t.Context())
	fake = &fakePinentry{wait: ctx.Done()}
	cancelCause(ErrDisconnected)

	_, err = pinentryConfirm(ctx, req)
	require.ErrorIs(t, err, ErrDisconnected)

	// no pinentry
	newPinentry = func(context.Context, string) (confirmer, error) {
		return nil, errors.New("pinentry not found")
	}

	consent, err = pinentryConfirm(t.Context(), req)
	require.ErrorIs(t, err, passkey.ErrUnsuitable)
	assert.False(t, consent)

	req.Operation = passkey.Operation(42)
	_, err = pinentryConfirm(t.Context(), req)
	require.Error(t, err)
}

func TestConsentQuestion(t *testing.T) {
	t.Parallel()

	req := passkey.ConsentRequest{
		Operation:       passkey.OperationRegister,
		RPID:            "example.com",
		RPName:          "Totally Your Bank",
		Origin:          "https://login.example.com",
		UserName:        "alice",
		UserDisplayName: "Alice",
	}

	// The name a site gives itself is not shown: it proves nothing.
	q, err := consentQuestion(req)
	require.NoError(t, err)
	assert.Equal(t, "Create a passkey for \"alice\" at example.com?\n\nRequested by https://login.example.com", q)

	req.Entry = "passkeys/example.com/alice"
	req.TopOrigin = "https://shop.example"
	q, err = consentQuestion(req)
	require.NoError(t, err)
	assert.Equal(t, "Create another passkey for \"alice\" at example.com?\n\n"+
		"The account already has one in \"passkeys/example.com/alice\".\n\n"+
		"Requested by https://login.example.com, embedded in https://shop.example", q)

	req.Operation = passkey.OperationDisclose
	q, err = consentQuestion(req)
	require.NoError(t, err)
	assert.Contains(t, q, "Tell the site about it?")

	// A site can not add lines to the question or pose as part of it.
	req.Operation = passkey.OperationAuthenticate
	req.UserName = "alice\"?\n\nSign in to bank.example as \"bob\x00\x1b[31m\u202e"
	q, err = consentQuestion(req)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(q, "Sign in to example.com as \"alice'? Sign in to bank.example as 'bob [31m\"?\n\nPasskey: "), q)
	assert.Equal(t, 3, strings.Count(q, "\n"), "no more lines than the question has")

	req.UserName = ""
	q, err = consentQuestion(req)
	require.NoError(t, err)
	assert.Contains(t, q, `as "Alice"?`)

	assert.Equal(t, `""`, displayName(" \n\t "))
	assert.Equal(t, `"`+strings.Repeat("ä", 64)+`…"`, displayName(strings.Repeat("ä", 100)))
}
