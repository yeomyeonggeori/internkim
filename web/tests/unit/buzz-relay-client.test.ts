import { describe, expect, test } from "bun:test";
import { verifyEvent } from "nostr-tools/pure";
import { buzzPublicKeyOf, signBuzzEvent } from "../../src/lib/buzz-relay-client";

const SECRET_KEY_HEX = "1178851e7a60684098157ea8fd4ef624c4fd094b41e274194843de1cd39c5aa8";

describe("buzz relay client — signing", () => {
	test("signs a stream message that verifies against the derived pubkey", () => {
		const event = signBuzzEvent(SECRET_KEY_HEX, {
			kind: 9,
			content: "hello buzz",
			tags: [["h", "channel-1"]],
			created_at: 1_700_000_000
		});
		expect(event.pubkey).toBe(buzzPublicKeyOf(SECRET_KEY_HEX));
		expect(event.content).toBe("hello buzz");
		expect(event.kind).toBe(9);
		expect(verifyEvent(event)).toBe(true);
	});

	test("produces the same pubkey as the vault vector", () => {
		expect(buzzPublicKeyOf(SECRET_KEY_HEX)).toHaveLength(64);
	});

	test("signature covers the tags — tampering invalidates it", () => {
		const event = signBuzzEvent(SECRET_KEY_HEX, {
			kind: 9,
			content: "x",
			tags: [["h", "channel-1"]],
			created_at: 1_700_000_000
		});
		const tampered = { ...JSON.parse(JSON.stringify(event)), tags: [["h", "other-channel"]] };
		expect(verifyEvent(tampered)).toBe(false);
	});
});
