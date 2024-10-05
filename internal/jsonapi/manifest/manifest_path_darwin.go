package manifest

var manifestPaths = &manifestPath{
	local: map[string]string{
		"arc":      "~/Library/Application Support/Arc/User Data/NativeMessagingHosts",
		"brave":    "~/Library/Application Support/Brave/NativeMessagingHosts",
		"chrome":   "~/Library/Application Support/Google/Chrome/NativeMessagingHosts",
		"chromium": "~/Library/Application Support/Chromium/NativeMessagingHosts",
		"firefox":  "~/Library/Application Support/Mozilla/NativeMessagingHosts",
		"iridium":  "~/Library/Application Support/Iridium/NativeMessagingHosts",
		"slimjet":  "~/Library/Application Support/Slimjet/NativeMessagingHosts",
		"vivaldi":  "~/Library/Application Support/Vivaldi/NativeMessagingHosts",
	},
	global: map[string]string{
		"arc":      "/Library/Application Support/Arc/User Data/NativeMessagingHosts",
		"brave":    "/Library/Application Support/Brave/NativeMessagingHosts",
		"chrome":   "/Library/Google/Chrome/NativeMessagingHosts",
		"chromium": "/Library/Application Support/Chromium/NativeMessagingHosts",
		"firefox":  "/Library/Application Support/Mozilla/NativeMessagingHosts",
		"iridium":  "/Library/Application Support/Iridium/NativeMessagingHosts",
		"slimjet":  "/Library/Application Support/Slimjet/NativeMessagingHosts",
		"vivaldi":  "/Library/Application Support/Vivaldi/NativeMessagingHosts",
	},
}
