import {
	unwrapSecretWithPasskey,
	unwrapSecretWithPassword,
	wrapSecretWithPasskey,
	wrapSecretWithPassword,
	type WrappedSecret,
} from "./buzz-key-vault";

export type BuzzClaim = { secretHex: string; publicHex: string };
export type BuzzVaultLookup = { found: boolean; wrapped?: WrappedSecret };

// The transport talks to admind: claim hands the key once (gated on Cloudflare
// Access), the vault stores/returns only the sealed blob admind cannot open.
export type BuzzIdentityTransport = {
	claim(): Promise<BuzzClaim>;
	fetchVault(): Promise<BuzzVaultLookup>;
	storeVault(wrapped: WrappedSecret): Promise<void>;
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

async function sealSecret(secretHex: string, factor: BuzzUnlockFactor): Promise<WrappedSecret> {
	return factor.kind === "password"
		? wrapSecretWithPassword(secretHex, factor.password)
		: wrapSecretWithPasskey(secretHex, factor.output);
}

async function openSecret(wrapped: WrappedSecret, factor: BuzzUnlockFactor): Promise<string> {
	return factor.kind === "password"
		? unwrapSecretWithPassword(wrapped, factor.password)
		: unwrapSecretWithPasskey(wrapped, factor.output);
}

// First device/enrollment: no sealed identity yet, so claim the key, seal it
// with the user's chosen factor, and store the opaque blob. Returns the secret
// held only in memory for signing.
export async function enrollBuzzIdentity(transport: BuzzIdentityTransport, factor: BuzzUnlockFactor): Promise<string> {
	const claim = await transport.claim();
	await transport.storeVault(await sealSecret(claim.secretHex, factor));
	return claim.secretHex;
}

// Returning device: a sealed identity exists, so fetch and open it with the
// user's factor. Throws BuzzIdentityNotEnrolledError when nothing is stored yet.
export async function unlockBuzzIdentity(transport: BuzzIdentityTransport, factor: BuzzUnlockFactor): Promise<string> {
	const vault = await transport.fetchVault();
	if (!vault.found || !vault.wrapped) {
		throw new BuzzIdentityNotEnrolledError();
	}
	return openSecret(vault.wrapped, factor);
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
			return (await response.json()) as BuzzVaultLookup;
		},
		async storeVault(wrapped) {
			const response = await fetch(`${root}/agent/api/buzz-vault`, {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify(wrapped),
			});
			if (!response.ok) throw new Error(`buzz vault store returned ${response.status}`);
		},
	};
}
