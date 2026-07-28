import { describe, expect, test } from "bun:test";
import {
	BuzzIdentityNotEnrolledError,
	BuzzRecoveryUnavailableError,
	enrollBuzzIdentity,
	hasEnrolledBuzzIdentity,
	recoverBuzzIdentity,
	unlockBuzzIdentity,
	type BuzzIdentityTransport,
	type BuzzUnlockFactor,
	type BuzzVaultDocument,
} from "../../src/lib/buzz-identity-session";

const SECRET_KEY_HEX = "1178851e7a60684098157ea8fd4ef624c4fd094b41e274194843de1cd39c5aa8";

function fakeTransport(): BuzzIdentityTransport & { stored: BuzzVaultDocument | null; claimCount: number } {
	const state = { stored: null as BuzzVaultDocument | null, claimCount: 0 };
	return {
		stored: null,
		claimCount: 0,
		async claim() {
			state.claimCount += 1;
			this.claimCount = state.claimCount;
			return { secretHex: SECRET_KEY_HEX, publicHex: "pub" };
		},
		async fetchVault() {
			return state.stored ? { found: true, document: state.stored } : { found: false };
		},
		async storeVault(document) {
			state.stored = document;
			this.stored = document;
		},
	};
}

const passwordFactor: BuzzUnlockFactor = { kind: "password", password: "correct horse" };

describe("buzz identity session", () => {
	test("enrolls and unlocks with a password", async () => {
		const transport = fakeTransport();
		expect(await enrollBuzzIdentity(transport, passwordFactor)).toBe(SECRET_KEY_HEX);
		expect(await unlockBuzzIdentity(transport, passwordFactor)).toBe(SECRET_KEY_HEX);
	});

	test("stores only opaque sealed copies, never the plaintext secret", async () => {
		const transport = fakeTransport();
		await enrollBuzzIdentity(transport, passwordFactor, "AAAA-BBBB-CCCC-DDDD-EEEE-FFFF");
		expect(JSON.stringify(transport.stored)).not.toContain(SECRET_KEY_HEX);
	});

	test("enrolls with a passkey and unlocks without re-claiming", async () => {
		const transport = fakeTransport();
		const factor: BuzzUnlockFactor = { kind: "passkey", output: crypto.getRandomValues(new Uint8Array(32)) };
		await enrollBuzzIdentity(transport, factor);
		expect(transport.claimCount).toBe(1);
		expect(await unlockBuzzIdentity(transport, factor)).toBe(SECRET_KEY_HEX);
		expect(transport.claimCount).toBe(1);
	});

	test("unlock before enrollment throws not-enrolled", async () => {
		await expect(unlockBuzzIdentity(fakeTransport(), passwordFactor)).rejects.toBeInstanceOf(BuzzIdentityNotEnrolledError);
	});

	test("hasEnrolled reflects whether a vault exists", async () => {
		const transport = fakeTransport();
		expect(await hasEnrolledBuzzIdentity(transport)).toBe(false);
		await enrollBuzzIdentity(transport, passwordFactor);
		expect(await hasEnrolledBuzzIdentity(transport)).toBe(true);
	});
});

describe("buzz identity recovery", () => {
	const recoveryCode = "AAAA-BBBB-CCCC-DDDD-EEEE-FFFF";

	test("recovers the same secret with the recovery code and re-seals a new factor", async () => {
		const transport = fakeTransport();
		await enrollBuzzIdentity(transport, passwordFactor, recoveryCode);
		const newFactor: BuzzUnlockFactor = { kind: "password", password: "new device password" };
		expect(await recoverBuzzIdentity(transport, recoveryCode, newFactor)).toBe(SECRET_KEY_HEX);
		// the new primary factor now unlocks, the old one no longer does
		expect(await unlockBuzzIdentity(transport, newFactor)).toBe(SECRET_KEY_HEX);
		await expect(unlockBuzzIdentity(transport, passwordFactor)).rejects.toThrow();
	});

	test("recovery is normalized so spacing and case do not matter", async () => {
		const transport = fakeTransport();
		await enrollBuzzIdentity(transport, passwordFactor, recoveryCode);
		expect(await recoverBuzzIdentity(transport, " aaaa bbbb cccc dddd eeee ffff ", passwordFactor)).toBe(SECRET_KEY_HEX);
	});

	test("recovery fails with the wrong code", async () => {
		const transport = fakeTransport();
		await enrollBuzzIdentity(transport, passwordFactor, recoveryCode);
		await expect(recoverBuzzIdentity(transport, "ZZZZ-ZZZZ-ZZZZ-ZZZZ-ZZZZ-ZZZZ", passwordFactor)).rejects.toThrow();
	});

	test("recovery throws when no recovery copy was stored", async () => {
		const transport = fakeTransport();
		await enrollBuzzIdentity(transport, passwordFactor);
		await expect(recoverBuzzIdentity(transport, recoveryCode, passwordFactor)).rejects.toBeInstanceOf(BuzzRecoveryUnavailableError);
	});
});
