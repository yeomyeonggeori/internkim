import { finalizeEvent, getPublicKey, type Event, type EventTemplate } from "nostr-tools/pure";

const STREAM_MESSAGE_KIND = 9;
const AUTH_KIND = 22242;
const AUTH_FALLBACK_MS = 1000;
const PUBLISH_TIMEOUT_MS = 8000;

function hexToBytes(hex: string): Uint8Array {
	const bytes = new Uint8Array(hex.length / 2);
	for (let index = 0; index < bytes.length; index++) {
		bytes[index] = Number.parseInt(hex.slice(index * 2, index * 2 + 2), 16);
	}
	return bytes;
}

export function buzzPublicKeyOf(secretHex: string): string {
	return getPublicKey(hexToBytes(secretHex));
}

export function signBuzzEvent(
	secretHex: string,
	template: { kind: number; content: string; tags: string[][]; created_at?: number }
): Event {
	const event: EventTemplate = {
		kind: template.kind,
		content: template.content,
		tags: template.tags,
		created_at: template.created_at ?? Math.floor(Date.now() / 1000)
	};
	return finalizeEvent(event, hexToBytes(secretHex));
}

function authEventTags(relayURL: string, challenge: string, extraAuthTag?: string[]): string[][] {
	const tags: string[][] = [
		["relay", relayURL],
		["challenge", challenge]
	];
	if (extraAuthTag) tags.push(extraAuthTag);
	return tags;
}

export type PublishBuzzMessageOptions = {
	channelId: string;
	content: string;
	replyToRootId?: string;
	extraTags?: string[][];
	extraAuthTag?: string[];
	timeoutMs?: number;
};

// Publishes a channel message to the Buzz relay signed by the person's own key
// in the browser. NIP-42 AUTH is answered with the same key. Single-shot: it
// opens a socket, authenticates, publishes, resolves on the relay's OK, and
// closes — the message never passes through the server signed as someone else.
export function publishBuzzMessage(
	relayURL: string,
	secretHex: string,
	options: PublishBuzzMessageOptions
): Promise<string> {
	return new Promise<string>((resolve, reject) => {
		const tags: string[][] = [["h", options.channelId], ...(options.extraTags ?? [])];
		if (options.replyToRootId) tags.push(["e", options.replyToRootId, "", "reply"]);
		const event = signBuzzEvent(secretHex, { kind: STREAM_MESSAGE_KIND, content: options.content, tags });

		const socket = new WebSocket(relayURL);
		let settled = false;
		let eventSent = false;

		const timeout = setTimeout(() => finish(new Error("relay publish timed out")), options.timeoutMs ?? PUBLISH_TIMEOUT_MS);
		let authFallback: ReturnType<typeof setTimeout> | undefined;

		function finish(error?: Error) {
			if (settled) return;
			settled = true;
			clearTimeout(timeout);
			if (authFallback) clearTimeout(authFallback);
			try {
				socket.close();
			} catch {
				// closing a failed socket is best-effort
			}
			if (error) reject(error);
			else resolve(event.id);
		}

		function sendEvent() {
			if (eventSent) return;
			eventSent = true;
			socket.send(JSON.stringify(["EVENT", event]));
		}

		socket.onopen = () => {
			// Most Buzz relays challenge on connect; if none arrives, publish anyway.
			authFallback = setTimeout(sendEvent, AUTH_FALLBACK_MS);
		};
		socket.onmessage = (message) => {
			let frame: unknown[];
			try {
				frame = JSON.parse(String(message.data));
			} catch {
				return;
			}
			const [type, ...rest] = frame;
			if (type === "AUTH" && typeof rest[0] === "string") {
				if (authFallback) clearTimeout(authFallback);
				const authEvent = signBuzzEvent(secretHex, {
					kind: AUTH_KIND,
					content: "",
					tags: authEventTags(relayURL, rest[0], options.extraAuthTag)
				});
				socket.send(JSON.stringify(["AUTH", authEvent]));
				sendEvent();
				return;
			}
			if (type === "OK" && rest[0] === event.id) {
				if (rest[1] === true) finish();
				else finish(new Error(`relay rejected event: ${String(rest[2] ?? "")}`));
			}
		};
		socket.onerror = () => finish(new Error("relay connection failed"));
	});
}
