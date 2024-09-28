package manifest

var manifestPaths = &manifestPath{
	local: map[string]string{
		"firefox":  "~/Library/Application Support/Mozilla/NativeMessagingHosts",
		"chrome":   "~/Library/Application Support/Google/Chrome/NativeMessagingHosts",
		"chromium": "~/Library/Application Support/Chromium/NativeMessagingHosts",
		"brave":    "~/Library/Application Support/Brave/NativeMessagingHosts",
		"vivaldi":  "~/Library/Application Support/Vivaldi/NativeMessagingHosts",
		"iridium":  "~/Library/Application Support/Iridium/NativeMessagingHosts",
		"slimjet":  "~/Library/Application Support/Slimjet/NativeMessagingHosts",
		"arc": 	    "~/Library/Application Support/Arc/User Data/NativeMessagingHosts",
	},
	global: map[string]string{
		"firefox":  "/Library/Application Support/Mozilla/NativeMessagingHosts",
		"chrome":   "/Library/Google/Chrome/NativeMessagingHosts",
		"chromium": "/Library/Application Support/Chromium/NativeMessagingHosts",
		"brave":    "/Library/Application Support/Brave/NativeMessagingHosts",
		"vivaldi":  "/Library/Application Support/Vivaldi/NativeMessagingHosts",
		"iridium":  "/Library/Application Support/Iridium/NativeMessagingHosts",
		"slimjet":  "/Library/Application Support/Slimjet/NativeMessagingHosts",
	},
}
