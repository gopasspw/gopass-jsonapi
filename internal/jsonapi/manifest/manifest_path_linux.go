package manifest

var manifestPaths = &manifestPath{
	local: map[string]string{
		"firefox":  "~/.mozilla/native-messaging-hosts",
		"floorp":   "~/.floorp/native-messaging-hosts",
		"chrome":   "~/.config/google-chrome/NativeMessagingHosts",
		"chromium": "~/.config/chromium/NativeMessagingHosts",
		"brave":    "~/.config/BraveSoftware/Brave-Browser/NativeMessagingHosts",
		"vivaldi":  "~/.config/vivaldi/NativeMessagingHosts",
		"iridium":  "~/.config/iridium/NativeMessagingHosts",
		"slimjet":  "~/.config/slimjet/NativeMessagingHosts",
	},
	global: map[string]string{
		"firefox":  "mozilla/native-messaging-hosts", // will be prefixed with the appropriate lib path
		"floorp":   "floorp/native-messaging-hosts",  // will be prefixed with the appropriate lib path
		"chrome":   "/etc/opt/chrome/native-messaging-hosts",
		"chromium": "/etc/chromium/native-messaging-hosts",
		"brave":    "/etc/opt/chrome/native-messaging-hosts",
		"vivaldi":  "/etc/opt/vivaldi/native-messaging-hosts",
		"iridium":  "/etc/iridium-browser/native-messaging-hosts",
		"slimjet":  "/etc/opt/slimjet/native-messaging-hosts",
	},
}
