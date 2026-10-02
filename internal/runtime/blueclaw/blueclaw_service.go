package blueclaw

import (
	"fmt"

	"github.com/yeomyeonggeori/internkim/internal/runtime/locallm"
)

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

func LlamaCppServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=internkim llama.cpp Server
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
User=root
Environment=LD_LIBRARY_PATH=%s
ExecStart=%s -m %s --model-draft %s --spec-type draft-mtp --spec-draft-n-max 4 --host %s --port %s -ngl 99 --spec-draft-ngl 99 -fa on --chat-template gemma --log-disable
Restart=on-failure
RestartSec=2
TimeoutStartSec=180

[Install]
WantedBy=multi-user.target
`, locallm.LlamaCppLibraryDir, locallm.LlamaCppBinaryPath, locallm.LlamaCppModelPath, locallm.LlamaCppDraftModelPath, locallm.LlamaCppHost, locallm.LlamaCppPort)
}

func LlamaCppEmbeddingServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=internkim llama.cpp Embedding Server
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
User=root
Environment=LD_LIBRARY_PATH=%s
ExecStart=%s -m %s --host %s --port %s -ngl 0 --embeddings --pooling cls --batch-size %s --ubatch-size %s --log-disable
Restart=on-failure
RestartSec=2
TimeoutStartSec=120

[Install]
WantedBy=multi-user.target
`, locallm.LlamaCppLibraryDir, locallm.LlamaCppBinaryPath, locallm.LlamaCppEmbeddingModelPath, locallm.LlamaCppHost, locallm.LlamaCppEmbeddingPort, locallm.LlamaCppEmbeddingBatchSize, locallm.LlamaCppEmbeddingUBatchSize)
}
