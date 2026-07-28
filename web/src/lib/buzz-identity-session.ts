import {
	unwrapSecretWithPasskey,
	unwrapSecretWithPassword,
	unwrapSecretWithRecoveryCode,
	wrapSecretWithPasskey,
	wrapSecretWithPassword,
	wrapSecretWithRecoveryCode,
	type WrappedSecret,
} from "./buzz-key-vault";

export type BuzzClaim = { secretHex: string; publicHex: string };
export type BuzzVaultDocument = { copies: WrappedSecret[] };
export type BuzzVaultLookup = { found: boolean; document?: BuzzVaultDocument };

// The transport talks to admind: claim hands the key once (gated on Cloudflare
// Access), the vault stores/returns only the sealed copies admind cannot open.
export type BuzzIdentityTransport = {
	claim(): Promise<BuzzClaim>;
	fetchVault(): Promise<BuzzVaultLookup>;
	storeVault(document: BuzzVaultDocument): Promise<void>;
};

export type BuzzUnlockFactor =
	| { kind: "password"; password: string }
	| { kind: "passkey"; output: Uint8Array };

export class BuzzIdentityNotEnrolledError extends Error {
	constructor() {
		super("no sealed Buzz identity is stored for this account yet");
		this.name = "BuzzIdentityNotEnrolledError";
	}
}

export class BuzzRecoveryUnavailableError extends Error {
	constructor() {
		super("no recovery copy is stored for this identity");
		this.name = "BuzzRecoveryUnavailableError";
	}
}

async function sealPrimary(secretHex: string, factor: BuzzUnlockFactor): Promise<WrappedSecret> {
	return factor.kind === "password"
		? wrapSecretWithPassword(secretHex, factor.password)
		: wrapSecretWithPasskey(secretHex, factor.output);
}

async function openPrimary(wrapped: WrappedSecret, factor: BuzzUnlockFactor): Promise<string> {
	return factor.kind === "password"
		? unwrapSecretWithPassword(wrapped, factor.password)
		: unwrapSecretWithPasskey(wrapped, factor.output);
}

// First device/enrollment: claim the key, seal it with the user's chosen factor
// and (when given) with a recovery code, and store the opaque copies. Returns
// the secret held only in memory for signing.
export async function enrollBuzzIdentity(
	transport: BuzzIdentityTransport,
	factor: BuzzUnlockFactor,
	recoveryCode?: string,
): Promise<string> {
	const claim = await transport.claim();
	const copies: WrappedSecret[] = [await sealPrimary(claim.secretHex, factor)];
	if (recoveryCode) {
		copies.push(await wrapSecretWithRecoveryCode(claim.secretHex, recoveryCode));
	}
	await transport.storeVault({ copies });
	return claim.secretHex;
}

function copyOfKind(document: BuzzVaultDocument | undefined, kind: WrappedSecret["kind"]): WrappedSecret | undefined {
	return document?.copies.find((copy) => copy.kind === kind);
}

// Returning device: open the copy sealed with the user's factor.
export async function unlockBuzzIdentity(transport: BuzzIdentityTransport, factor: BuzzUnlockFactor): Promise<string> {
	const vault = await transport.fetchVault();
	if (!vault.found || !vault.document) {
		throw new BuzzIdentityNotEnrolledError();
	}
	const copy = copyOfKind(vault.document, factor.kind);
	if (!copy) {
		throw new BuzzIdentityNotEnrolledError();
	}
	return openPrimary(copy, factor);
}

// Recovery: open the recovery-sealed copy with the user's recovery code, then
// re-seal under a fresh primary factor (and a fresh recovery code) so the lost
// device is replaced without changing the identity or losing history.
export async function recoverBuzzIdentity(
	transport: BuzzIdentityTransport,
	recoveryCode: string,
	newFactor: BuzzUnlockFactor,
	newRecoveryCode?: string,
): Promise<string> {
	const vault = await transport.fetchVault();
	const copy = copyOfKind(vault.document, "recovery");
	if (!vault.found || !copy) {
		throw new BuzzRecoveryUnavailableError();
	}
	const secretHex = await unwrapSecretWithRecoveryCode(copy, recoveryCode);
	const copies: WrappedSecret[] = [await sealPrimary(secretHex, newFactor)];
	copies.push(await wrapSecretWithRecoveryCode(secretHex, newRecoveryCode ?? recoveryCode));
	await transport.storeVault({ copies });
	return secretHex;
}

export async function hasEnrolledBuzzIdentity(transport: BuzzIdentityTransport): Promise<boolean> {
	return (await transport.fetchVault()).found;
}

export function createBuzzIdentityTransport(baseURL = ""): BuzzIdentityTransport {
	const root = baseURL.endsWith("/") ? baseURL.slice(0, -1) : baseURL;
	return {
		async claim() {
			const response = await fetch(`${root}/agent/api/buzz-claim`, { method: "POST" });
			if (!response.ok) throw new Error(`buzz claim returned ${response.status}`);
			return (await response.json()) as BuzzClaim;
		},
		async fetchVault() {
			const response = await fetch(`${root}/agent/api/buzz-vault`);
			if (!response.ok) throw new Error(`buzz vault fetch returned ${response.status}`);
			const document = (await response.json()) as { found: boolean; wrapped?: BuzzVaultDocument };
			return { found: document.found, document: document.wrapped };
		},
		async storeVault(document) {
			const response = await fetch(`${root}/agent/api/buzz-vault`, {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify(document),
			});
			if (!response.ok) throw new Error(`buzz vault store returned ${response.status}`);
		},
	};
}
