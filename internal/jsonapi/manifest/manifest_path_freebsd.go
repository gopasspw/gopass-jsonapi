package manifest

var manifestPaths = &manifestPath{
	local: map[string]string{
		"firefox":  "~/.mozilla/native-messaging-hosts",
		"chrome":   "~/.config/google-chrome/NativeMessagingHosts",
		"chromium": "~/.config/chromium/NativeMessagingHosts",
		// "brave":    "~/.config/BraveSoftware/Brave-Browser/NativeMessagingHosts",
		// "vivaldi":  "~/.config/vivaldi/NativeMessagingHosts",
		"iridium":  "~/.config/iridium/NativeMessagingHosts",
		// "slimjet":  "~/.config/slimjet/NativeMessagingHosts",
	},
	global: map[string]string{
		"firefox":  "/usr/local/lib/mozilla/native-messaging-hosts", 
		"chrome":   "/usr/local/lib/chrome/native-messaging-hosts",
		"chromium": "/usr/local/lib/chromium/native-messaging-hosts",
		// "brave":    "/usr/local/lib/chrome/native-messaging-hosts",
		// "vivaldi":  "/usr/local/lib/vivaldi/native-messaging-hosts",
		"iridium":  "/usr/local/lib/iridium-browser/native-messaging-hosts",
		// "slimjet":  "/usr/local/lib/slimjet/native-messaging-hosts",
	},
}
