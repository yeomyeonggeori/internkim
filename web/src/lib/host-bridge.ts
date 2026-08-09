import { supabase } from '$lib/supabase';
import { messengerCredential } from '$lib/messenger-credential';
import type { RealtimeChannel } from '@supabase/supabase-js';

export type HostCall = {
	capability: string;
	body?: Record<string, unknown>;
};

export type HostAnswer = {
	status: number;
	body: unknown;
};

export class HostUnreachableError extends Error {
	constructor() {
		super('the company app is not running');
		this.name = 'HostUnreachableError';
	}
}

type Wire = {
	memberID: string;
	presence: RealtimeChannel;
	mine: RealtimeChannel;
};

const answerTimeoutMilliseconds = 20_000;

let joined: Promise<Wire> | undefined;
let running = false;
const waiting = new Map<string, (answer: HostAnswer) => void>();

async function companyWire(): Promise<Wire> {
	if (joined) return joined;
	const attempt = openWire();
	joined = attempt;
	attempt.catch(() => forgetWire(attempt));
	return attempt;
}

function forgetWire(attempt: Promise<Wire>): void {
	if (joined !== attempt) return;
	joined = undefined;
	running = false;
}

async function openWire(): Promise<Wire> {
	const client = supabase();
	await client.realtime.setAuth();
	const { data } = await client.auth.getSession();
	const accountID = data.session?.user.id;
	if (!accountID) throw new Error('sign in first');
	const member = await client
		.from('member')
		.select('id, company_id')
		.eq('user_id', accountID)
		.single<{ id: string; company_id: string }>();
	if (member.error) throw new Error(member.error.message);

	const presence = client.channel(`company:${member.data.company_id}`, {
		config: { private: true }
	});
	presence.on('presence', { event: 'sync' }, () => {
		running = Object.keys(presence.presenceState()).length > 0;
	});
	await joinChannel(presence);

	const mine = client.channel(`member:${member.data.id}`, { config: { private: true } });
	mine.on('broadcast', { event: 'answer' }, ({ payload }) => {
		const answer = payload as { callID?: string; status?: number; body?: unknown };
		if (typeof answer.callID !== 'string') return;
		waiting.get(answer.callID)?.({ status: answer.status ?? 500, body: answer.body });
		waiting.delete(answer.callID);
	});
	await joinChannel(mine);

	return { memberID: member.data.id, presence, mine };
}

function joinChannel(channel: RealtimeChannel): Promise<void> {
	return new Promise<void>((resolve, reject) => {
		channel.subscribe((status, error) => {
			if (status === 'SUBSCRIBED') return resolve();
			if (error) return reject(error);
			if (status === 'CHANNEL_ERROR' || status === 'TIMED_OUT' || status === 'CLOSED') {
				reject(new Error(`the ${channel.topic} channel is ${status}`));
			}
		});
	});
}

export async function isCompanyAppRunning(): Promise<boolean> {
	await companyWire();
	return running;
}

export function callPayload(
	callID: string,
	memberID: string,
	call: HostCall,
	actor: { kind: string; secret: string }
): Record<string, unknown> {
	return {
		callID,
		capability: call.capability,
		replyTo: memberID,
		body: { ...call.body, actor }
	};
}

export async function callCompanyApp(call: HostCall): Promise<HostAnswer> {
	const wire = await companyWire();
	const actor = await messengerCredential();

	const callID = crypto.randomUUID();
	const answered = new Promise<HostAnswer | null>((resolve) => {
		waiting.set(callID, resolve);
		setTimeout(() => {
			if (!waiting.delete(callID)) return;
			resolve(null);
		}, answerTimeoutMilliseconds);
	});

	await wire.mine.send({
		type: 'broadcast',
		event: 'call',
		payload: callPayload(callID, wire.memberID, call, actor)
	});

	const answer = await answered;
	if (!answer) throw new HostUnreachableError();
	return answer;
}
