package manifest

var manifestPaths = &manifestPath{
	local: map[string]string{
		"firefox":   "~/Library/Application Support/Mozilla/NativeMessagingHosts",
		"librewolf": "~/Library/Application Support/Librewolf/NativeMessagingHosts",
		"chrome":    "~/Library/Application Support/Google/Chrome/NativeMessagingHosts",
		"chromium":  "~/Library/Application Support/Chromium/NativeMessagingHosts",
		"brave":     "~/Library/Application Support/Brave/NativeMessagingHosts",
		"vivaldi":   "~/Library/Application Support/Vivaldi/NativeMessagingHosts",
		"iridium":   "~/Library/Application Support/Iridium/NativeMessagingHosts",
		"slimjet":   "~/Library/Application Support/Slimjet/NativeMessagingHosts",
	},
	global: map[string]string{
		"firefox":  "/Library/Application Support/Mozilla/NativeMessagingHosts",
		"librewolf": "/Library/Application Support/Librewolf/NativeMessagingHosts",
		"chrome":   "/Library/Google/Chrome/NativeMessagingHosts",
		"chromium": "/Library/Application Support/Chromium/NativeMessagingHosts",
		"brave":    "/Library/Application Support/Brave/NativeMessagingHosts",
		"vivaldi":  "/Library/Application Support/Vivaldi/NativeMessagingHosts",
		"iridium":  "/Library/Application Support/Iridium/NativeMessagingHosts",
		"slimjet":  "/Library/Application Support/Slimjet/NativeMessagingHosts",
	},
}
