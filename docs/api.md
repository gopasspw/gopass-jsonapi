# API Overview

The API follows the specification for native messaging from [Mozilla](https://developer.mozilla.org/en-US/Add-ons/WebExtensions/Native_messaging) and [Chrome](https://developer.chrome.com/apps/nativeMessaging).
Each JSON-UTF8 encoded message is prefixed with a 32-bit integer specifying the length of the message.
Communication is performed via stdin/stdout.

**WARNING**: This API **MUST NOT** be exposed over the network to remote hosts.
**No authentication is performed** and the only safe way is to communicate via stdin/stdout as you do in your terminal.

## Request Types

### `query`

#### Request

```json
{
  "type": "query",
  "query": "secret"
}
```

#### Response

```json
[
    "somewhere/mysecret/loginname",
    "somewhere/else/secretsauce"
]
```

### `queryHost`

Similar to `query` but cuts hostnames and subdomains from the left side until the response to the query is non-empty. Stops if only the [public suffix](https://publicsuffix.org/) is remaining.

#### Request

```json
{
  "type": "queryHost",
  "host": "some.domain.example.com"
}
```

#### Response:

```json
[
    "somewhere/domain.example.com/loginname",
    "somewhere/other.domain.example.com"
]
```

### `getLogin`

#### Request

```json
{
   "type": "getLogin",
   "entry": "somewhere/else/secretsauce"
}
```

#### Response:

```json
{
   "username": "hugo",
   "password": "thepassword"
}
```

### `getData`

#### Request

```json
{
   "type": "getData",
   "entry": "somewhere/else/secretsauce"
}
```

#### Response:

```json
{
   "current_totp": "576548"
}
```

### `create`

#### Request

```json
{
   "type": "create",
   "login": "myusername",
   "password": "",
   "length": 12,
   "generate": true,
   "use_symbols": true
}
```

#### Response:

```json
{
   "username": "myusername",
   "password": "5^dX9j1\"b5^q"
}
```

## Passkeys

`gopass-jsonapi` can act as a [WebAuthn](https://www.w3.org/TR/webauthn-3/) authenticator: it creates passkeys, keeps them in the password store and signs in with them.
The three requests below let a browser extension answer the calls a web page makes to `navigator.credentials.create()` and `navigator.credentials.get()`.

The `options` of the requests are the `publicKey` options of the web page in their JSON form, i.e. [`PublicKeyCredentialCreationOptionsJSON`](https://www.w3.org/TR/webauthn-3/#dictdef-publickeycredentialcreationoptionsjson) and [`PublicKeyCredentialRequestOptionsJSON`](https://www.w3.org/TR/webauthn-3/#dictdef-publickeycredentialrequestoptionsjson): all binary values are base64url encoded without padding.
The `credential` of the responses is what `PublicKeyCredential.toJSON()` returns, i.e. [`RegistrationResponseJSON`](https://www.w3.org/TR/webauthn-3/#dictdef-registrationresponsejson) and [`AuthenticationResponseJSON`](https://www.w3.org/TR/webauthn-3/#dictdef-authenticationresponsejson). It includes the `clientDataJSON`, which is assembled by `gopass-jsonapi`.

Each request names the document that made the call:

| Field          | Description                                                                                                         |
|----------------|---------------------------------------------------------------------------------------------------------------------|
| `origin`       | The origin of the document, e.g. `https://login.example.com`, as the browser reports it.                           |
| `cross_origin` | `true` if the document is not same-origin with all of its ancestors, i.e. embedded in a document of another origin. Optional. |
| `top_origin`   | The origin of the top-level document. Required if `cross_origin` is `true`.                                        |

A passkey can only be used by the site it was created for. `gopass-jsonapi` enforces this based on the `origin`: the RP ID of a request must be the domain of the origin or a parent domain of it that is not a public suffix. Passkeys are therefore not handed out to phishing sites, provided that the origin is correct.

**WARNING**: The extension is responsible for the following. Getting it wrong defeats the protection passkeys offer.

* The `origin`, `cross_origin` and `top_origin` **MUST** be determined by the extension itself, from the browser, and **MUST NOT** be taken from the web page.
* Requests **MUST** only be forwarded if the browser would allow the call: the document is a secure context and, if it is embedded in a document of another origin, the [Permissions Policy](https://www.w3.org/TR/webauthn-3/#sctn-permissions-policy) permits `publickey-credentials-create` or `publickey-credentials-get`. Creating a passkey from such a document additionally requires a user activation.
* Of a failed request, the web page **MUST NOT** learn more than the `error_name`. The `error` text and the result of `passkeyQuery` are for the user. They tell which accounts exist.
* A request that fails because there is no passkey **SHOULD NOT** be answered faster than one the user declines. If `fallback` is `true`, hand the call to the WebAuthn implementation of the browser.

The private key of a passkey never leaves `gopass-jsonapi`. The other requests do not return it either: `getData` leaves out the `passkey` field of a secret and `copyToClipboard` refuses to copy it.

### User consent

No passkey is created or used without the consent of the user. `gopass-jsonapi` asks for it itself, with the `pinentry` program that is also used to ask for passphrases, so that neither a web page nor the extension can sign in on behalf of the user unnoticed. The requests `passkeyCreate` and `passkeyGet` therefore block until the user answered, until the `timeout` of the options has passed, or until the browser closes the connection. The timeout is at least 5 and at most 10 minutes. If there is no way to ask the user, e.g. because there is no `pinentry` or it can not show a dialog, the requests fail with `fallback` set.

A browser keeps the connection to `gopass-jsonapi` open until it has the response. If it closes the connection before, the request ends without a response and the dialog is taken down. An extension that wants to be able to cancel a request, e.g. because the web page passed an `AbortSignal` or went away, sends the request with `runtime.connectNative()` and disconnects the port to cancel it. `runtime.sendNativeMessage()` offers no way to do so. In either case `gopass-jsonapi` answers a single message.

A confirmation shows that the user is present. It does not verify who the user is, so the `UV` flag is not set and requests that require user verification fail with `fallback` set, unless the user enabled the gopass config option `passkey.user-verification`. See the [gopass documentation](https://github.com/gopasspw/gopass/blob/master/docs/commands/passkey.md) for details and for how passkeys are stored.

### `passkeyCreate`

Creates a passkey, stores it and returns the response for the relying party. This answers `navigator.credentials.create()`.

A passkey the account already has is kept. The response is only sent once the new passkey is stored.

#### Request

```json
{
  "type": "passkeyCreate",
  "origin": "https://login.example.com",
  "options": {
    "rp": { "id": "example.com", "name": "Example Inc." },
    "user": { "id": "aGFuZGxlLWFsaWNl", "name": "alice", "displayName": "Alice" },
    "challenge": "cmVnaXN0ZXI",
    "pubKeyCredParams": [{ "type": "public-key", "alg": -7 }],
    "excludeCredentials": [],
    "authenticatorSelection": { "residentKey": "required", "userVerification": "preferred" },
    "extensions": { "credProps": true }
  }
}
```

#### Response

```json
{
  "entry": "passkeys/example.com/alice",
  "credential": {
    "id": "3yZL-uu9GR2Vmm6_61WyIlXIfjNzKWZB8bfjMX3fWmI",
    "rawId": "3yZL-uu9GR2Vmm6_61WyIlXIfjNzKWZB8bfjMX3fWmI",
    "type": "public-key",
    "authenticatorAttachment": "platform",
    "response": {
      "clientDataJSON": "eyJ0eXBlIjoid2ViYXV0aG4uY3JlYXRlIiwi...",
      "authenticatorData": "o3mm9u6vuaVeN4wRgDTidR5oL6ufLTCrE9IS...",
      "transports": ["internal"],
      "publicKey": "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...",
      "publicKeyAlgorithm": -7,
      "attestationObject": "o2NmbXRkbm9uZWdhdHRTdG10oGhhdXRoRGF0..."
    },
    "clientExtensionResults": { "credProps": { "rk": true } }
  }
}
```

`entry` is the name of the secret the passkey was stored in.

The response has an additional `warning` if the passkey was stored but the password store reported a failure afterwards, typically because it could not push to its remote. The registration is complete in that case. The passkey just is not synced to other devices yet, which the user should be told.

The algorithms `-7` (ES256), `-8` (EdDSA with Ed25519) and `-257` (RS256) are supported. The passkey is always discoverable. No attestation is provided: the attestation statement format is `none`.

### `passkeyQuery`

Lists the passkeys that can answer a request of `navigator.credentials.get()`, so that the extension can let the user pick an account. Nothing is signed and the user is not asked for consent. The secrets holding the passkeys of the relying party are decrypted to do so.

If the options have `allowCredentials`, only those passkeys are listed. The request fails with `fallback` set if there is none, and also if `passkeyGet` would refuse the request because it requires a user verification that is not enabled.

#### Request

```json
{
  "type": "passkeyQuery",
  "origin": "https://login.example.com",
  "options": {
    "challenge": "c2lnbiBpbg",
    "rpId": "example.com",
    "allowCredentials": [],
    "userVerification": "preferred"
  }
}
```

#### Response

```json
{
  "credentials": [
    {
      "id": "3yZL-uu9GR2Vmm6_61WyIlXIfjNzKWZB8bfjMX3fWmI",
      "entry": "passkeys/example.com/alice",
      "user_name": "alice",
      "user_display_name": "Alice"
    }
  ]
}
```

### `passkeyGet`

Signs the challenge of the relying party with a passkey and returns the response for the relying party. This answers `navigator.credentials.get()`.

`credential_id` is the `id` of the passkey the user picked from the result of `passkeyQuery`. It is optional if only one passkey matches. If more than one matches and none is selected the request fails.

#### Request

```json
{
  "type": "passkeyGet",
  "origin": "https://login.example.com",
  "credential_id": "3yZL-uu9GR2Vmm6_61WyIlXIfjNzKWZB8bfjMX3fWmI",
  "options": {
    "challenge": "c2lnbiBpbg",
    "rpId": "example.com",
    "allowCredentials": [],
    "userVerification": "preferred"
  }
}
```

#### Response

```json
{
  "entry": "passkeys/example.com/alice",
  "credential": {
    "id": "3yZL-uu9GR2Vmm6_61WyIlXIfjNzKWZB8bfjMX3fWmI",
    "rawId": "3yZL-uu9GR2Vmm6_61WyIlXIfjNzKWZB8bfjMX3fWmI",
    "type": "public-key",
    "authenticatorAttachment": "platform",
    "response": {
      "clientDataJSON": "eyJ0eXBlIjoid2ViYXV0aG4uZ2V0Iiwi...",
      "authenticatorData": "o3mm9u6vuaVeN4wRgDTidR5oL6ufLTCrE9ISVYbOGUcZAAAAAA",
      "signature": "MEUCIQDx...",
      "userHandle": "aGFuZGxlLWFsaWNl"
    },
    "clientExtensionResults": {}
  }
}
```

As with `passkeyCreate`, the response can have a `warning`. That is the case if the passkey maintains a signature counter and the password store reported a failure after the counter was written.

### Passkey errors

A failed passkey request is answered with the name of the [`DOMException`](https://www.w3.org/TR/webauthn-3/#sctn-create-request-exceptions) the promise of the WebAuthn API is to be rejected with:

```json
{
  "error": "NotAllowedError: no matching passkey found for example.com",
  "error_name": "NotAllowedError",
  "fallback": true
}
```

| `error_name`        | Meaning                                                                                                       |
|---------------------|---------------------------------------------------------------------------------------------------------------|
| `NotAllowedError`   | The user declined, there is no matching passkey, or anything else the relying party is not to tell apart.     |
| `InvalidStateError` | A passkey listed in `excludeCredentials` is already stored, and the user agreed to let the site know.         |
| `SecurityError`     | The origin is not allowed to use the RP ID.                                                                   |
| `NotSupportedError` | None of the algorithms in `pubKeyCredParams` is supported.                                                    |
| `TypeError`         | The request or its options are malformed.                                                                     |
| `UnknownError`      | Something else went wrong, e.g. the passkey could not be stored.                                              |

`fallback` is `true` if the request is not one for gopass: there is no passkey for it, the relying party asks for something gopass does not offer, i.e. a security key (`authenticatorAttachment: "cross-platform"`), an algorithm that is not supported, or user verification that is not enabled, or there is no way to ask the user for consent. The extension should then hand the call to the WebAuthn implementation of the browser.

A request that can not be read at all, e.g. because it is not valid JSON, is answered with the plain [error response](#error-response) instead.

## Error Response

If an uncaught error occurs, the stringified error message is send back as the response:

```json
{
  "error": "Some error occurred with fancy message"
}
```
