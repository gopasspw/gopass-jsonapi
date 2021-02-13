# JSON API

Note: The gopass JSON API has been moved to its own binary and separate repository.

This is still work-in-progress and no regular release process has been defined.
You might encounter outdated or incomplete documentation across different gopasspw repositories.

## Installation

You may need to manually download the `gopass-jsonapi` binary from GitHub, until it is available in popular package managers.

Alternatively you can compile it yourself if you have Go 1.14 (or greater) installed:

```bash
go get github.com/gopasspw/gopass-jsonapi
```

## API Overview

The API follows the specification for native messaging from [Mozilla](https://developer.mozilla.org/en-US/Add-ons/WebExtensions/Native_messaging) and [Chrome](https://developer.chrome.com/apps/nativeMessaging).
Each JSON-UTF8 encoded message is prefixed with a 32-bit integer specifying the length of the message.
Communication is performed via stdin/stdout.

**WARNING**: This API **MUST NOT** be exposed over the network to remote hosts.
**No authentication is performed** and the only safe way is to communicate via stdin/stdout as you do in your terminal.

The implementation is located in `utils/jsonapi`.

## Request Types

### `query`

#### Query:

```json
{
  "type": "query",
  "query": "secret"
}
```

#### Response:

```json
[
    "somewhere/mysecret/loginname",
    "somewhere/else/secretsauce"
]
```

### `queryHost`

Similar to `query` but cuts hostnames and subdomains from the left side until the response to the query is non-empty. Stops if only the [public suffix](https://publicsuffix.org/) is remaining.

#### Query:

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

#### Query:

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

### `create`

#### Query:

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

## Error Response

If an uncaught error occurs, the stringified error message is send back as the response:

```json
{
  "error": "Some error occurred with fancy message"
}
```
Gopass Browser Bindings
