import { describe, expect, test } from "bun:test";
import {
	generateRecoveryCode,
	normalizeRecoveryCode,
	unwrapSecretWithPasskey,
	unwrapSecretWithPassword,
	unwrapSecretWithRecoveryCode,
	wrapSecretWithPasskey,
	wrapSecretWithPassword,
	wrapSecretWithRecoveryCode,
	type WrappedSecret,
} from "../../src/lib/buzz-key-vault";

const SECRET_KEY_HEX = "1178851e7a60684098157ea8fd4ef624c4fd094b41e274194843de1cd39c5aa8";

describe("buzz key vault — password", () => {
	test("round-trips the secret key through a password", async () => {
		const wrapped = await wrapSecretWithPassword(SECRET_KEY_HEX, "correct horse battery staple");
		expect(await unwrapSecretWithPassword(wrapped, "correct horse battery staple")).toBe(SECRET_KEY_HEX);
	});

	test("does not store the plaintext secret in the wrapped blob", async () => {
		const wrapped = await wrapSecretWithPassword(SECRET_KEY_HEX, "pw");
		expect(JSON.stringify(wrapped)).not.toContain(SECRET_KEY_HEX);
	});

	test("fails to unwrap with the wrong password", async () => {
		const wrapped = await wrapSecretWithPassword(SECRET_KEY_HEX, "right");
		await expect(unwrapSecretWithPassword(wrapped, "wrong")).rejects.toThrow();
	});

	test("uses a fresh salt and iv per wrap so two blobs differ", async () => {
		const first = await wrapSecretWithPassword(SECRET_KEY_HEX, "pw");
		const second = await wrapSecretWithPassword(SECRET_KEY_HEX, "pw");
		expect(first.ciphertext).not.toBe(second.ciphertext);
		expect(first.salt).not.toBe(second.salt);
	});
});

describe("buzz key vault — passkey", () => {
	test("round-trips the secret key through a passkey PRF output", async () => {
		const passkeyOutput = crypto.getRandomValues(new Uint8Array(32));
		const wrapped = await wrapSecretWithPasskey(SECRET_KEY_HEX, passkeyOutput);
		expect(await unwrapSecretWithPasskey(wrapped, passkeyOutput)).toBe(SECRET_KEY_HEX);
	});

	test("fails to unwrap with a different passkey output", async () => {
		const wrapped = await wrapSecretWithPasskey(SECRET_KEY_HEX, crypto.getRandomValues(new Uint8Array(32)));
		await expect(unwrapSecretWithPasskey(wrapped, crypto.getRandomValues(new Uint8Array(32)))).rejects.toThrow();
	});
});

describe("buzz key vault — recovery code", () => {
	test("generates a grouped code from the unambiguous alphabet", () => {
		const code = generateRecoveryCode();
		expect(code).toMatch(/^[A-Z0-9]{4}(-[A-Z0-9]{4}){5}$/);
		expect(code).not.toMatch(/[ILO01]/);
	});

	test("round-trips the secret through a recovery code, ignoring spacing and case", async () => {
		const wrapped = await wrapSecretWithRecoveryCode(SECRET_KEY_HEX, "ABCD-EFGH-JKLM-NPQR-STUV-WXYZ");
		expect(wrapped.kind).toBe("recovery");
		expect(await unwrapSecretWithRecoveryCode(wrapped, " abcd efgh jklm npqr stuv wxyz ")).toBe(SECRET_KEY_HEX);
	});

	test("normalizes to uppercase alphanumerics", () => {
		expect(normalizeRecoveryCode(" ab-cd ef ")).toBe("ABCDEF");
	});

	test("a recovery blob does not open as a password", async () => {
		const wrapped = await wrapSecretWithRecoveryCode(SECRET_KEY_HEX, "ABCD-EFGH-JKLM-NPQR-STUV-WXYZ");
		await expect(unwrapSecretWithPassword(wrapped, "ABCD-EFGH-JKLM-NPQR-STUV-WXYZ")).rejects.toThrow();
	});
});

describe("buzz key vault — validation", () => {
	test("rejects a malformed secret key", async () => {
		await expect(wrapSecretWithPassword("not-hex", "pw")).rejects.toThrow();
	});

	test("rejects unwrapping a password blob as a passkey", async () => {
		const wrapped: WrappedSecret = await wrapSecretWithPassword(SECRET_KEY_HEX, "pw");
		await expect(unwrapSecretWithPasskey(wrapped, new Uint8Array(32))).rejects.toThrow();
	});
});
