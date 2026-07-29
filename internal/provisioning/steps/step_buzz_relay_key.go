package setup

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const buzzRelayKeyDevicePath = "/root/.internkim/secrets/buzz-relay-env"
const buzzRelayKeyStagedPath = "secrets/buzz-relay-env"

// StepBuzzRelayKey provisions the relay's own signing key. Unlike the identity
// seed, this key has no off-device source — the host owns it. The step
// generates it once on the device, never overwrites an existing one (a changed
// relay key invalidates every membership grant it has signed), and never
// surfaces it. It is written as an EnvironmentFile the relay service loads.
var StepBuzzRelayKey = Step{
	Name: "buzz-relay-key",
	Title: func(context *Context) string {
		return context.T("Buzz 릴레이 키 설정...", "Configuring Buzz relay key...")
	},
	IsSatisfied: func(context *Context) bool {
		switch context.Backend {
		case BackendSSH:
			return buzzKeySeedIsPresent(trimmedRun(context, "cat "+buzzRelayKeyDevicePath+" 2>/dev/null || true"))
		case BackendSD:
			document, errorValue := readStagedFile(context, buzzRelayKeyStagedPath)
			return errorValue == nil && buzzKeySeedIsPresent(string(document))
		}
		return false
	},
	Run: func(context *Context) error {
		context.SSH.Run(`mkdir -p /root/.internkim/secrets
chown root:root /root/.internkim/secrets
chmod 700 /root/.internkim/secrets
if [ ! -s ` + buzzRelayKeyDevicePath + ` ]; then
  printf 'BUZZ_RELAY_PRIVATE_KEY=%s\n' "$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')" > ` + buzzRelayKeyDevicePath + `
fi
chown root:root ` + buzzRelayKeyDevicePath + `
chmod 600 ` + buzzRelayKeyDevicePath)
		fmt.Println("  " + context.T("릴레이 키 설치 완료", "Relay key installed"))
		return nil
	},
	RunSD: func(context *Context) error {
		key, errorValue := randomRelayKeyHex()
		if errorValue != nil {
			return errorValue
		}
		if errorValue := context.SD.WriteFile(buzzRelayKeyStagedPath, []byte("BUZZ_RELAY_PRIVATE_KEY="+key+"\n"), 0o600); errorValue != nil {
			return errorValue
		}
		fmt.Println("  " + context.T("릴레이 키 스테이지 완료", "Relay key staged"))
		return nil
	},
}

func randomRelayKeyHex() (string, error) {
	keyBytes := make([]byte, 32)
	if _, errorValue := rand.Read(keyBytes); errorValue != nil {
		return "", errorValue
	}
	return hex.EncodeToString(keyBytes), nil
}
