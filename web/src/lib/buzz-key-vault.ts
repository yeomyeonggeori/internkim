export type WrappedSecretKind = "password" | "passkey";

export type WrappedSecret = {
	version: 1;
	kind: WrappedSecretKind;
	ciphertext: string;
	initializationVector: string;
	salt?: string;
	iterations?: number;
};

const AES_KEY_LENGTH_BITS = 256;
const INITIALIZATION_VECTOR_LENGTH_BYTES = 12;
const PASSWORD_SALT_LENGTH_BYTES = 16;
const PASSWORD_DERIVATION_ITERATIONS = 600_000;
const SECRET_KEY_HEX_LENGTH = 64;

function encodeBase64(bytes: Uint8Array): string {
	let binary = "";
	for (const byte of bytes) binary += String.fromCharCode(byte);
	return btoa(binary);
}

function decodeBase64(value: string): Uint8Array {
	const binary = atob(value);
	const bytes = new Uint8Array(binary.length);
	for (let index = 0; index < binary.length; index++) bytes[index] = binary.charCodeAt(index);
	return bytes;
}

function hexToBytes(hex: string): Uint8Array {
	const bytes = new Uint8Array(hex.length / 2);
	for (let index = 0; index < bytes.length; index++) {
		bytes[index] = Number.parseInt(hex.slice(index * 2, index * 2 + 2), 16);
	}
	return bytes;
}

function bytesToHex(bytes: Uint8Array): string {
	let hex = "";
	for (const byte of bytes) hex += byte.toString(16).padStart(2, "0");
	return hex;
}

function validateSecretKeyHex(secretKeyHex: string): void {
	if (!/^[0-9a-f]{64}$/.test(secretKeyHex.toLowerCase())) {
		throw new Error(`buzz secret key must be ${SECRET_KEY_HEX_LENGTH} lowercase hex characters`);
	}
}

async function deriveUnlockKeyFromPassword(
	password: string,
	salt: Uint8Array,
	iterations: number,
): Promise<CryptoKey> {
	const passwordKey = await crypto.subtle.importKey(
		"raw",
		new TextEncoder().encode(password),
		"PBKDF2",
		false,
		["deriveKey"],
	);
	return crypto.subtle.deriveKey(
		{ name: "PBKDF2", salt: salt as BufferSource, iterations, hash: "SHA-256" },
		passwordKey,
		{ name: "AES-GCM", length: AES_KEY_LENGTH_BITS },
		false,
		["encrypt", "decrypt"],
	);
}

async function importUnlockKeyFromPasskeyOutput(passkeyOutput: Uint8Array): Promise<CryptoKey> {
	const digest = await crypto.subtle.digest("SHA-256", passkeyOutput as BufferSource);
	return crypto.subtle.importKey("raw", digest, { name: "AES-GCM" }, false, ["encrypt", "decrypt"]);
}

async function sealSecret(secretKeyHex: string, unlockKey: CryptoKey): Promise<{ ciphertext: string; initializationVector: string }> {
	validateSecretKeyHex(secretKeyHex);
	const initializationVector = crypto.getRandomValues(new Uint8Array(INITIALIZATION_VECTOR_LENGTH_BYTES));
	const sealed = await crypto.subtle.encrypt(
		{ name: "AES-GCM", iv: initializationVector as BufferSource },
		unlockKey,
		hexToBytes(secretKeyHex.toLowerCase()) as BufferSource,
	);
	return {
		ciphertext: encodeBase64(new Uint8Array(sealed)),
		initializationVector: encodeBase64(initializationVector),
	};
}

async function openSecret(wrapped: WrappedSecret, unlockKey: CryptoKey): Promise<string> {
	const plaintext = await crypto.subtle.decrypt(
		{ name: "AES-GCM", iv: decodeBase64(wrapped.initializationVector) as BufferSource },
		unlockKey,
		decodeBase64(wrapped.ciphertext) as BufferSource,
	);
	return bytesToHex(new Uint8Array(plaintext));
}

// The server stores WrappedSecret as an opaque blob it cannot open. Only the
// user's password or passkey-derived unlock key decrypts the Buzz secret key,
// and only transiently in the browser at signing time.
export async function wrapSecretWithPassword(secretKeyHex: string, password: string): Promise<WrappedSecret> {
	const salt = crypto.getRandomValues(new Uint8Array(PASSWORD_SALT_LENGTH_BYTES));
	const unlockKey = await deriveUnlockKeyFromPassword(password, salt, PASSWORD_DERIVATION_ITERATIONS);
	const sealed = await sealSecret(secretKeyHex, unlockKey);
	return {
		version: 1,
		kind: "password",
		ciphertext: sealed.ciphertext,
		initializationVector: sealed.initializationVector,
		salt: encodeBase64(salt),
		iterations: PASSWORD_DERIVATION_ITERATIONS,
	};
}

export async function unwrapSecretWithPassword(wrapped: WrappedSecret, password: string): Promise<string> {
	if (wrapped.kind !== "password" || !wrapped.salt || !wrapped.iterations) {
		throw new Error("wrapped secret was not sealed with a password");
	}
	const unlockKey = await deriveUnlockKeyFromPassword(password, decodeBase64(wrapped.salt), wrapped.iterations);
	return openSecret(wrapped, unlockKey);
}

export async function wrapSecretWithPasskey(secretKeyHex: string, passkeyOutput: Uint8Array): Promise<WrappedSecret> {
	const unlockKey = await importUnlockKeyFromPasskeyOutput(passkeyOutput);
	const sealed = await sealSecret(secretKeyHex, unlockKey);
	return {
		version: 1,
		kind: "passkey",
		ciphertext: sealed.ciphertext,
		initializationVector: sealed.initializationVector,
	};
}

export async function unwrapSecretWithPasskey(wrapped: WrappedSecret, passkeyOutput: Uint8Array): Promise<string> {
	if (wrapped.kind !== "passkey") {
		throw new Error("wrapped secret was not sealed with a passkey");
	}
	const unlockKey = await importUnlockKeyFromPasskeyOutput(passkeyOutput);
	return openSecret(wrapped, unlockKey);
}
