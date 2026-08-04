import { supabase } from '$lib/supabase';
import type { RealtimeChannel } from '@supabase/supabase-js';

export type HostCall = {
	method: 'GET' | 'POST' | 'PUT' | 'DELETE';
	path: string;
	body?: unknown;
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

const answerTimeoutMilliseconds = 20_000;

let joined: Promise<RealtimeChannel> | undefined;
let running = false;
const waiting = new Map<string, (answer: HostAnswer) => void>();

async function companyChannel(): Promise<RealtimeChannel> {
	joined ??= (async () => {
		const client = supabase();
		await client.realtime.setAuth();
		const { data } = await client.auth.getSession();
		const accountID = data.session?.user.id;
		if (!accountID) throw new Error('sign in first');
		const member = await client
			.from('member')
			.select('company_id')
			.eq('user_id', accountID)
			.single<{ company_id: string }>();
		if (member.error) throw new Error(member.error.message);

		const channel = client.channel(`company:${member.data.company_id}`, { config: { private: true } });
		channel.on('presence', { event: 'sync' }, () => {
			running = Object.keys(channel.presenceState()).length > 0;
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
		return channel;
	})();
	return joined;
}

export async function isCompanyAppRunning(): Promise<boolean> {
	await companyChannel();
	return running;
}

export async function callCompanyApp(call: HostCall): Promise<HostAnswer> {
	const channel = await companyChannel();

	const callID = crypto.randomUUID();
	const answered = new Promise<HostAnswer | null>((resolve) => {
		waiting.set(callID, resolve);
		setTimeout(() => {
			if (!waiting.delete(callID)) return;
			resolve(null);
		}, answerTimeoutMilliseconds);
	});

	await channel.send({
		type: 'broadcast',
		event: 'call',
		payload: { callID, method: call.method, path: call.path, body: call.body ?? null }
	});

	const answer = await answered;
	if (!answer) throw new HostUnreachableError();
	return answer;
}
