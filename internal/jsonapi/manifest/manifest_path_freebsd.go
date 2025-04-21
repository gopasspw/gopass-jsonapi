package manifest

var manifestPaths = &manifestPath{
	local: map[string]string{
		"firefox":            "~/.mozilla/native-messaging-hosts",
		"chrome":             "~/.config/google-chrome/NativeMessagingHosts",
		"chromium":           "~/.config/chromium/NativeMessagingHosts",
		"ungoogled-chromium": "~/.config/ungoogled-chromium/NativeMessagingHosts",
		"iridium":            "~/.config/iridium/NativeMessagingHosts",
	},
	global: map[string]string{
		"firefox":            "/usr/local/lib/mozilla/native-messaging-hosts",
		"chrome":             "/usr/local/lib/chrome/native-messaging-hosts",
		"chromium":           "/usr/local/lib/chromium/native-messaging-hosts",
		"ungoogled-chromium": "/usr/local/lib/ungoogled-chromium/native-messaging-hosts",
		"iridium":            "/usr/local/lib/iridium-browser/native-messaging-hosts",
	},
}
