// Waits for the agent to say something back. A refusal counts as an answer —
// the bundle thought about it and replied — so this only fails on silence,
// which is what a misconfigured bundle produces.
import { finalizeEvent } from 'nostr-tools/pure';

const relayURL = required('SMOKE_RELAY_URL');
const channelID = required('SMOKE_CHANNEL_ID');
const agentPubkey = required('SMOKE_AGENT_PUBKEY').toLowerCase();
const waitSeconds = Number(process.env.SMOKE_WAIT_SECONDS ?? '90');
const senderSecret = await readSecret(required('SMOKE_SENDER_KEY_PATH'));
const askedAt = Math.floor(Date.now() / 1000) - 30;

function required(name: string): string {
	const value = process.env[name]?.trim();
	if (!value) throw new Error(`set ${name}`);
	return value;
}

async function readSecret(path: string): Promise<Uint8Array> {
	const text = (await Bun.file(path).text()).trim();
	return Uint8Array.from(Buffer.from(text, 'hex'));
}

const socket = new WebSocket(relayURL);

socket.addEventListener('message', (message) => {
	const frame = JSON.parse(String(message.data)) as unknown[];
	if (frame[0] === 'AUTH' && typeof frame[1] === 'string') {
		socket.send(
			JSON.stringify([
				'AUTH',
				finalizeEvent(
					{
						kind: 22242,
						created_at: Math.floor(Date.now() / 1000),
						content: '',
						tags: [
							['relay', relayURL],
							['challenge', frame[1]]
						]
					},
					senderSecret
				)
			])
		);
		return;
	}
	if (frame[0] === 'OK') {
		socket.send(
			JSON.stringify([
				'REQ',
				'smoke',
				{ kinds: [9, 40003], '#h': [channelID], authors: [agentPubkey], since: askedAt }
			])
		);
		return;
	}
	if (frame[0] !== 'EVENT') return;
	const event = frame[2] as { content?: string };
	console.log(`[smoke] the agent answered: ${String(event.content ?? '').slice(0, 200)}`);
	socket.close();
	process.exit(0);
});

setTimeout(() => {
	console.error(`[smoke] no answer in ${waitSeconds}s — the bundle is up but cannot answer`);
	process.exit(1);
}, waitSeconds * 1000);
