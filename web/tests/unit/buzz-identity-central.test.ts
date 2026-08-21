import { describe, expect, test } from "bun:test";
import { centralBuzzIdentityTransport } from "../../src/lib/buzz-identity-central";
import type { WrappedSecret } from "../../src/lib/buzz-key-vault";

const ACCOUNT_ID = "b9b6d0a8-0f22-4a4a-9a0f-1d0a2a4d9a11";
const MEMBER_ID = "3f4a2c11-8e77-4f2e-9a01-5d3c6b8e0a22";

const sealedPassword: WrappedSecret = {
	version: 1,
	kind: "password",
	ciphertext: "sealed-by-password",
	initializationVector: "iv",
	salt: "salt",
	iterations: 600_000,
};

function fakeSupabase(rows: { kind: string; settings: WrappedSecret }[]) {
	const written: Record<string, unknown>[] = [];
	const client = {
		written,
		from(table: string) {
			if (table === "member") {
				return {
					select: () => ({
						eq: () => ({ single: async () => ({ data: { id: MEMBER_ID }, error: null }) }),
					}),
				};
			}
			return {
				select: () => ({ eq: () => ({ like: async () => ({ data: rows, error: null }) }) }),
				upsert: async (values: Record<string, unknown>[]) => {
					written.push(...values);
					return { error: null };
				},
			};
		},
	};
	return client;
}

describe("a company member's sealed Buzz identity", () => {
	test("reads only the vault rows, not every credential the member has", async () => {
		const client = fakeSupabase([{ kind: "buzz-vault-password", settings: sealedPassword }]);

		const lookup = await centralBuzzIdentityTransport(client as never, ACCOUNT_ID).fetchVault();

		expect(lookup.found).toBe(true);
		expect(lookup.document?.copies).toEqual([sealedPassword]);
	});

	test("an account with no vault is not enrolled rather than an error", async () => {
		const client = fakeSupabase([]);

		expect(await centralBuzzIdentityTransport(client as never, ACCOUNT_ID).fetchVault()).toEqual({
			found: false,
		});
	});

	test("stores one row per unlock factor and never touches the server-readable vault", async () => {
		const client = fakeSupabase([]);

		await centralBuzzIdentityTransport(client as never, ACCOUNT_ID).storeVault({
			copies: [sealedPassword, { ...sealedPassword, kind: "passkey" }],
		});

		expect(client.written.map((row) => row.kind)).toEqual([
			"buzz-vault-password",
			"buzz-vault-passkey",
		]);
		expect(client.written.every((row) => row.member_id === MEMBER_ID)).toBe(true);
		expect(client.written.every((row) => row.vault_secret_id === null)).toBe(true);
	});

	test("mints a key in the browser, so no server ever holds an openable secret", async () => {
		const claim = await centralBuzzIdentityTransport(fakeSupabase([]) as never, ACCOUNT_ID).claim();

		expect(claim.secretHex).toMatch(/^[0-9a-f]{64}$/);
		expect(claim.publicHex).toMatch(/^[0-9a-f]{64}$/);
	});

	test("mints a different key every time", async () => {
		const transport = centralBuzzIdentityTransport(fakeSupabase([]) as never, ACCOUNT_ID);

		expect((await transport.claim()).secretHex).not.toBe((await transport.claim()).secretHex);
	});
});
