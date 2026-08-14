// Puts one message in the conversation as a member, signed with their own key,
// the way their messenger would. Nothing here talks to the agent directly: if
// the bundle is wired wrongly the message simply goes unanswered, which is the
// failure this is looking for.
import { finalizeEvent } from 'nostr-tools/pure';

const relayURL = required('SMOKE_RELAY_URL');
const channelID = required('SMOKE_CHANNEL_ID');
const question = required('SMOKE_QUESTION');
const senderSecret = await readSecret(required('SMOKE_SENDER_KEY_PATH'));

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
let hasAsked = false;

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
	if (frame[0] !== 'OK') return;
	if (!hasAsked) {
		hasAsked = true;
		socket.send(
			JSON.stringify([
				'EVENT',
				finalizeEvent(
					{
						kind: 9,
						created_at: Math.floor(Date.now() / 1000),
						content: question,
						tags: [['h', channelID]]
					},
					senderSecret
				)
			])
		);
		return;
	}
	const accepted = frame[2] === true;
	console.log(accepted ? '[smoke] the relay took the question' : `[smoke] the relay refused it: ${JSON.stringify(frame)}`);
	socket.close();
	process.exit(accepted ? 0 : 1);
});

setTimeout(() => {
	console.error('[smoke] the relay never answered');
	process.exit(1);
}, 15000);
