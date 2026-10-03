package blueclaw

import "fmt"

// BuzzRelayReadinessURL is the one address anything asking "is the messenger
// ready" asks, on the device and on the packaged host alike.
func BuzzRelayReadinessURL() string {
	return "http://" + BuzzRelayBindAddress + BuzzRelayReadinessPath
}

func ChatdLegacyTLSDropInPaths() []string {
	return []string{ChatdDropInDirectory + "/tls.conf", ChatdDropInDirectory + "/tls-debug.conf"}
}

// BuzzMediaServiceUnit starts the S3 gateway the relay stores attachments
// through. The posix backend serves the bucket directory as-is, so no client
// creates it and nothing about the bucket lives inside a storage format.
//
// --versioning-dir is deliberately absent, and
// TestBuzzMediaUnitDoesNotEnableVersioning is why: without it the gateway
// refuses PutBucketVersioning outright, which makes the one incompatibility
// measured against versitygw 1.8.0 unreachable. Read that test before adding
// the flag.
func BuzzMediaServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Buzz Media Store
After=network-online.target
Wants=network-online.target

[Service]
User=root
EnvironmentFile=%s
ExecStart=%s --port %s --health %s posix %s
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, BuzzMediaEnvironmentFilePath, BuzzMediaBinaryPath, BuzzMediaAddress, BuzzMediaHealthPath, BuzzMediaRootPath)
}
