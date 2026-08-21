import { describe, expect, mock, test } from "bun:test";

const call = mock(async () => ({ status: 200, body: { secretHex: "a".repeat(64), publicHex: "b".repeat(64) } }));
mock.module("../../src/lib/host-bridge", () => ({ callCompanyApp: call }));

const { claimCentralBuzzSecret } = await import("../../src/lib/buzz-identity-central-login");

describe("a company browser claiming its Buzz key", () => {
	test("asks the machine that holds the seed rather than minting one", async () => {
		call.mockResolvedValueOnce({ status: 200, body: { secretHex: "c".repeat(64), publicHex: "d".repeat(64) } });

		expect(await claimCentralBuzzSecret()).toBe("c".repeat(64));
		expect(call.mock.calls.at(-1)?.[0]).toEqual({ capability: "person.buzz.claim" });
	});

	test("a refused claim is an error, not an empty identity", async () => {
		call.mockResolvedValueOnce({ status: 401, body: {} });

		expect(claimCentralBuzzSecret()).rejects.toThrow("401");
	});

	test("an answer carrying no key is an error rather than a blank sign-in", async () => {
		call.mockResolvedValueOnce({ status: 200, body: {} });

		expect(claimCentralBuzzSecret()).rejects.toThrow("carried no key");
	});
});
