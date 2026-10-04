import { describe, expect, test } from "bun:test";
import { verifyEvent } from "nostr-tools/pure";
import { buzzPublicKeyOf, publishBuzzMessage, signBuzzEvent, streamMessageTags } from "../../src/lib/buzz-relay-client";

const SECRET_KEY_HEX = "1178851e7a60684098157ea8fd4ef624c4fd094b41e274194843de1cd39c5aa8";

test('an expired identity is refused before any relay connection or signature', async () => {
	await expect(publishBuzzMessage('wss://relay.example.com', 'invalid-key', {
		channelId: 'channel-1', content: 'test', isCurrent: () => false
	})).rejects.toThrow('the signed-in identity changed');
});

test('an account change while connecting never sends relay authentication or a message', async () => {
	const originalSocket = globalThis.WebSocket;
	const sent: string[] = [];
	let receive: (event: { data: string }) => void = () => {};
	let closed = false;
	class Socket {
		set onmessage(handler: typeof receive) { receive = handler; }
		send(value: string) { sent.push(value); }
		close() { closed = true; }
	}
	Reflect.set(globalThis, 'WebSocket', Socket);
	try {
		let current = true;
		const pending = publishBuzzMessage('wss://relay.example.com', SECRET_KEY_HEX, {
			channelId: 'channel-1', content: 'test', isCurrent: () => current
		});
		current = false;
		receive({ data: JSON.stringify(['AUTH', 'sample-challenge']) });
		await expect(pending).rejects.toThrow('the signed-in identity changed');
		expect(sent).toEqual([]);
		expect(closed).toBe(true);
	} finally {
		Reflect.set(globalThis, 'WebSocket', originalSocket);
	}
});

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

describe("buzz relay client — thread tags", () => {
	test("a reply names the root it answers", () => {
		const tags = streamMessageTags({ channelId: "channel-1", content: "x", replyToRootId: "root-1" });
		expect(tags).toEqual([["h", "channel-1"], ["e", "root-1", "", "root"]]);
	});

	test("a message that answers nothing carries no thread tag", () => {
		const tags = streamMessageTags({ channelId: "channel-1", content: "x" });
		expect(tags).toEqual([["h", "channel-1"]]);
	});

	test("keeps the attachment tags it was given", () => {
		const imeta = ["imeta", "url https://example.com/a.png"];
		const tags = streamMessageTags({ channelId: "channel-1", content: "x", extraTags: [imeta] });
		expect(tags).toEqual([["h", "channel-1"], imeta]);
	});
});
