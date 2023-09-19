# JSON API

`gopass-jsonapi` enables communication with gopass via JSON messages.

This is particularly useful for browser plugins like [gopassbridge](https://github.com/gopasspw/gopassbridge) running gopass as native app.
More details can be found in [api.md](./docs/api.md).

## Project status

This is still work-in-progress and no regular release process has been defined.
You might encounter outdated or incomplete documentation across different repositories and gopass versions.

## Installation

**gopass v1.10 / v1.11**:

The binary for v1.10 and v1.11 can be downloaded and unpacked from
[archive files on Github Releases](https://github.com/gopasspw/gopass/releases/tag/v1.11.0).

**gopass v1.12 or newer**:

You need to manually download the `gopass-jsonapi` binary from [GitHub Releases](https://github.com/gopasspw/gopass-jsonapi/releases),
until it is available in popular package managers.

Alternatively you can compile it yourself if you have Go 1.14 (or greater) installed:

```bash
git clone https://github.com/gopasspw/gopass-jsonapi.git
cd gopass-jsonapi
make build
./gopass-jsonapi help
```

You need to run `gopass-jsonapi configure` for each browser you want to use with `gopass`.

**Fedora**:

The jsonapi is available in Fedora repositories, so you can simply install it with:

```bash
sudo dnf install gopass-jsonapi
```

**Windows**:

The jsonapi setup copies the current gopass-jsonapi binary as a wrapper executable file (`gopass_native_host.exe` calls the listener directly).
It is recommended to run `gopass-jsonapi configure` after each **update** to have the latest version setup for your browser.
The **global** setup requires to run `gopass-jsonapi configure` as Administrator.

## Usage

Gopass allows filling in passwords in browsers leveraging a browser plugin like [gopassbridge](https://github.com/gopasspw/gopassbridge).
The browser plugin communicates with gopass-jsonapi via JSON messages.
To allow the plugin to interact with gopass-jsonapi,
a [native messaging manifest](https://developer.mozilla.org/en-US/Add-ons/WebExtensions/Native_messaging) must be installed for each browser.

This native extension and the gopassbrigde plugin currently only support the *Connectionless messaging* with [`runtime.sendNativeMessage`](https://github.com/gopasspw/gopassbridge/blob/master/web-extension/generic.js#L54), i.e.
the `gopass-jsonapi` will be started for every single message from the brower plugin.

You need to run `gopass-jsonapi configure` to configure your browser for `gopass-jsonapi`.

```bash
# Asks all questions concerning browser and setup
gopass-jsonapi configure

# Do not copy / install any files, just print their location and content
gopass-jsonapi configure --print

# Specify browser and wrapper path
gopass-jsonapi configure --browser chrome --path /home/user/.local/
```

## How user name is determined

The user name/login is determined from `login`, `username` and `user` yaml attributes (after the --- separator) like this:

```yaml
<your password>
---
username: <your username>
```

As fallback, the last part of the path is used, e.g. `theuser1` for `Internet/github.com/theuser1` entry.

## Supported Browsers

### Linux / macOS

- Firefox
- Chrome
- Chromium
- Brave
- Vivaldi
- Iridium
- Slimjet
- Librewolf

### Windows

- Firefox
- Chrome
- Chromium
- Brave

## Contributing

Thank you very much for supporting gopass. Pull requests are welcome.

Please follow the [gopass contribution guidelines and Pull Request checklist](https://github.com/gopasspw/gopass/blob/master/CONTRIBUTING.md#pull-request-checklist).
