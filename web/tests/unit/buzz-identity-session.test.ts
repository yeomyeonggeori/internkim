import { describe, expect, test } from "bun:test";
import {
	BuzzIdentityNotEnrolledError,
	enrollBuzzIdentity,
	hasEnrolledBuzzIdentity,
	unlockBuzzIdentity,
	type BuzzIdentityTransport,
	type BuzzUnlockFactor,
} from "../../src/lib/buzz-identity-session";
import type { WrappedSecret } from "../../src/lib/buzz-key-vault";

const SECRET_KEY_HEX = "1178851e7a60684098157ea8fd4ef624c4fd094b41e274194843de1cd39c5aa8";

function fakeTransport(): BuzzIdentityTransport & { stored: WrappedSecret | null; claimCount: number } {
	const state = { stored: null as WrappedSecret | null, claimCount: 0 };
	return {
		stored: null,
		claimCount: 0,
		async claim() {
			state.claimCount += 1;
			this.claimCount = state.claimCount;
			return { secretHex: SECRET_KEY_HEX, publicHex: "pub" };
		},
		async fetchVault() {
			return state.stored ? { found: true, wrapped: state.stored } : { found: false };
		},
		async storeVault(wrapped) {
			state.stored = wrapped;
			this.stored = wrapped;
		},
	};
}

const passwordFactor: BuzzUnlockFactor = { kind: "password", password: "correct horse" };

describe("buzz identity session — password factor", () => {
	test("enrolls by claiming, sealing, and storing; then unlocks to the same secret", async () => {
		const transport = fakeTransport();
		const enrolled = await enrollBuzzIdentity(transport, passwordFactor);
		expect(enrolled).toBe(SECRET_KEY_HEX);
		expect(transport.stored).not.toBeNull();
		expect(await unlockBuzzIdentity(transport, passwordFactor)).toBe(SECRET_KEY_HEX);
	});

	test("stores only the opaque sealed blob, never the plaintext secret", async () => {
		const transport = fakeTransport();
		await enrollBuzzIdentity(transport, passwordFactor);
		expect(JSON.stringify(transport.stored)).not.toContain(SECRET_KEY_HEX);
	});

	test("unlock fails with the wrong password", async () => {
		const transport = fakeTransport();
		await enrollBuzzIdentity(transport, passwordFactor);
		await expect(unlockBuzzIdentity(transport, { kind: "password", password: "wrong" })).rejects.toThrow();
	});

	test("unlock before enrollment throws not-enrolled", async () => {
		const transport = fakeTransport();
		await expect(unlockBuzzIdentity(transport, passwordFactor)).rejects.toBeInstanceOf(BuzzIdentityNotEnrolledError);
	});

	test("hasEnrolled reflects whether a vault exists", async () => {
		const transport = fakeTransport();
		expect(await hasEnrolledBuzzIdentity(transport)).toBe(false);
		await enrollBuzzIdentity(transport, passwordFactor);
		expect(await hasEnrolledBuzzIdentity(transport)).toBe(true);
	});
});

describe("buzz identity session — passkey factor", () => {
	test("enrolls and unlocks with a passkey PRF output", async () => {
		const transport = fakeTransport();
		const output = crypto.getRandomValues(new Uint8Array(32));
		const factor: BuzzUnlockFactor = { kind: "passkey", output };
		await enrollBuzzIdentity(transport, factor);
		expect(await unlockBuzzIdentity(transport, factor)).toBe(SECRET_KEY_HEX);
	});

	test("a returning device unlocks without re-claiming", async () => {
		const transport = fakeTransport();
		await enrollBuzzIdentity(transport, passwordFactor);
		expect(transport.claimCount).toBe(1);
		await unlockBuzzIdentity(transport, passwordFactor);
		expect(transport.claimCount).toBe(1);
	});
});
