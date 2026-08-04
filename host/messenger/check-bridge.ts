//   bun run host/messenger/check-bridge.ts

import { createClient, type RealtimeChannel } from '@supabase/supabase-js';

type Answer = { status: number; body: unknown };

const projectURL = required('SUPABASE_URL');
const publishableKey = required('SUPABASE_PUBLISHABLE_KEY');
const memberEmail = required('HOST_MEMBER_EMAIL');
const memberPassword = required('HOST_MEMBER_PASSWORD');

function required(name: string): string {
	const value = process.env[name];
	if (!value) throw new Error(`set ${name}`);
	return value;
}

const client = createClient(projectURL, publishableKey);
const signedIn = await client.auth.signInWithPassword({ email: memberEmail, password: memberPassword });
if (signedIn.error) throw new Error(signedIn.error.message);

const member = await client
	.from('member')
	.select('id, company_id')
	.eq('user_id', signedIn.data.user.id)
	.single<{ id: string; company_id: string }>();
if (member.error) throw new Error(member.error.message);

await client.realtime.setAuth();
const waiting = new Map<string, (answer: Answer) => void>();
let appPresent = false;

const channel: RealtimeChannel = client.channel(`company:${member.data.company_id}`, { config: { private: true } });
channel.on('presence', { event: 'sync' }, () => {
	appPresent = Object.keys(channel.presenceState()).length > 0;
});
channel.on('broadcast', { event: 'answer' }, ({ payload }) => {
	const answer = payload as { callID?: string; status?: number; body?: unknown };
	if (typeof answer.callID !== 'string') return;
	waiting.get(answer.callID)?.({ status: answer.status ?? 500, body: answer.body });
	waiting.delete(answer.callID);
});
await new Promise<void>((resolve, reject) => {
	channel.subscribe((status, error) => {
		if (status === 'SUBSCRIBED') resolve();
		if (error) reject(error);
	});
});

async function call(method: string, path: string, body?: unknown): Promise<Answer> {
	const callID = crypto.randomUUID();
	const answered = new Promise<Answer | null>((resolve) => {
		waiting.set(callID, resolve);
		setTimeout(() => {
			if (waiting.delete(callID)) resolve(null);
		}, 20_000);
	});
	await channel.send({ type: 'broadcast', event: 'call', payload: { callID, method, path, body: body ?? null } });
	const answer = await answered;
	if (!answer) throw new Error(`${method} ${path}: the app did not answer`);
	if (answer.status >= 400) throw new Error(`${method} ${path}: ${JSON.stringify(answer.body)}`);
	return answer;
}

const checks: string[] = [];
function passed(what: string) {
	checks.push(what);
	console.log(`  ✓ ${what}`);
}

await new Promise((resolve) => setTimeout(resolve, 2000));
if (!appPresent) throw new Error('the company app is not connected');
passed('the app announces itself');

const channels = (await call('GET', '/channel')).body as {
	id: string;
	name: string;
	isDirect: boolean;
	position: number;
	participants: { externalID?: string }[];
}[];
if (channels.length === 0) throw new Error('no channels came back');
passed(`${channels.length} channels, ordered ${channels[0].position} first`);

const ordered = channels.every((entry, index) => entry.position === index);
if (!ordered) throw new Error('channel positions are not a running order');
passed('positions run in order');

const direct = channels.filter((entry) => entry.isDirect);
if (direct.length > 0 && direct.every((entry) => entry.participants.length === 0)) {
	throw new Error('direct channels came back with nobody in them');
}
passed(direct.length > 0 ? 'direct channels say who is in them' : 'no direct channels to check');

const room = channels.find((entry) => !entry.isDirect);
if (!room) throw new Error('no ordinary channel to write in');

const written = (await call('POST', `/channel/${room.id}/post`, { body: 'bridge check' })).body as { id: string };
passed(`wrote a post in ${room.name}`);

const edited = (await call('PUT', `/post/${written.id}`, { body: 'bridge check, edited' })).body as { body: string };
if (edited.body !== 'bridge check, edited') throw new Error('the edit did not take');
passed('edited it');

await call('POST', `/post/${written.id}/reaction`, { emoji: 'eyes' });
const withReaction = (await call('GET', `/channel/${room.id}/post`)).body as {
	id: string;
	reactions: { emoji: string }[];
}[];
const reacted = withReaction.find((post) => post.id === written.id);
if (!reacted?.reactions.some((reaction) => reaction.emoji === 'eyes')) throw new Error('the reaction did not come back');
passed('reacted, and the reaction reads back');

await call('DELETE', `/post/${written.id}/reaction?emoji=eyes`);
passed('took the reaction away');

await call('DELETE', `/post/${written.id}`);
const afterwards = (await call('GET', `/channel/${room.id}/post`)).body as { id: string }[];
if (afterwards.some((post) => post.id === written.id)) throw new Error('the post survived deletion');
passed('deleted it, and it is gone');

console.log(`\n${checks.length} checks passed`);
process.exit(0);
