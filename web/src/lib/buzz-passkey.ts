const PRF_SALT = new TextEncoder().encode("internkim-buzz-identity-prf-v1");
const CREDENTIAL_STORAGE_KEY = "internkim.buzz.passkey.credentialId";

// The PRF extension is not in every TypeScript lib.dom yet, so the extension
// inputs/outputs are described locally and merged at the call sites.
type PrfExtensionInput = { prf: { eval?: { first: BufferSource } } };
type PrfExtensionOutput = { prf?: { results?: { first?: ArrayBuffer } } };

function bytesToBase64url(bytes: Uint8Array): string {
	let binary = "";
	for (const byte of bytes) binary += String.fromCharCode(byte);
	return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function base64urlToBytes(value: string): Uint8Array {
	const padded = value.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(value.length / 4) * 4, "=");
	const binary = atob(padded);
	const bytes = new Uint8Array(binary.length);
	for (let index = 0; index < binary.length; index++) bytes[index] = binary.charCodeAt(index);
	return bytes;
}

export function isPasskeySupported(): boolean {
	return (
		typeof window !== "undefined" &&
		typeof window.PublicKeyCredential !== "undefined" &&
		typeof navigator !== "undefined" &&
		navigator.credentials != null
	);
}

export function hasRegisteredBuzzPasskey(): boolean {
	return typeof window !== "undefined" && window.localStorage.getItem(CREDENTIAL_STORAGE_KEY) != null;
}

// Registers a discoverable passkey with the PRF extension enabled. residentKey
// "required" makes it discoverable, so it syncs (iCloud/Google) and can unlock
// on another browser without any device-local state.
export async function registerBuzzPasskey(email: string, displayName: string): Promise<void> {
	const challenge = crypto.getRandomValues(new Uint8Array(32));
	const credential = (await navigator.credentials.create({
		publicKey: {
			challenge,
			rp: { name: "InternKim", id: window.location.hostname },
			user: { id: new TextEncoder().encode(email), name: email, displayName: displayName || email },
			pubKeyCredParams: [
				{ type: "public-key", alg: -7 },
				{ type: "public-key", alg: -257 },
			],
			authenticatorSelection: { residentKey: "required", userVerification: "preferred" },
			extensions: { prf: {} } as AuthenticationExtensionsClientInputs & PrfExtensionInput,
		},
	})) as PublicKeyCredential | null;
	if (!credential) throw new Error("passkey registration was cancelled");
	window.localStorage.setItem(CREDENTIAL_STORAGE_KEY, bytesToBase64url(new Uint8Array(credential.rawId)));
}

async function assertBuzzPasskey(): Promise<PublicKeyCredential> {
	const stored = window.localStorage.getItem(CREDENTIAL_STORAGE_KEY);
	const allowCredentials = stored
		? [{ id: base64urlToBytes(stored) as BufferSource, type: "public-key" as const }]
		: [];
	const assertion = (await navigator.credentials.get({
		publicKey: {
			challenge: crypto.getRandomValues(new Uint8Array(32)),
			rpId: window.location.hostname,
			allowCredentials,
			userVerification: "preferred",
			extensions: { prf: { eval: { first: PRF_SALT } } } as AuthenticationExtensionsClientInputs & PrfExtensionInput,
		},
	})) as PublicKeyCredential | null;
	if (!assertion) throw new Error("passkey unlock was cancelled");
	return assertion;
}

function passkeyPrfOutput(assertion: PublicKeyCredential): Uint8Array {
	const results = assertion.getClientExtensionResults() as PrfExtensionOutput;
	const first = results.prf?.results?.first;
	if (!first) throw new Error("this authenticator does not support the PRF extension");
	return new Uint8Array(first);
}

// Derives the passkey PRF output that seals/opens the Buzz key. Targets the
// stored credential when known, else lets the platform discover a synced
// passkey — so a new browser unlocks with no device-local state.
export async function deriveBuzzPasskeyOutput(): Promise<Uint8Array> {
	return passkeyPrfOutput(await assertBuzzPasskey());
}

// Usernameless login: a discoverable passkey identifies the person (its user
// handle is their email) and yields the unlock output in one gesture, so the
// login screen needs no typed email.
export async function loginWithBuzzPasskey(): Promise<{ email: string; output: Uint8Array }> {
	const assertion = await assertBuzzPasskey();
	const userHandle = (assertion.response as AuthenticatorAssertionResponse).userHandle;
	if (!userHandle) throw new Error("이 패스키에는 계정 정보가 없습니다. 이메일로 로그인하세요.");
	return { email: new TextDecoder().decode(userHandle), output: passkeyPrfOutput(assertion) };
}
