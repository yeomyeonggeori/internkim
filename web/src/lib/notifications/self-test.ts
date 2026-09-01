import { FunctionsHttpError } from '@supabase/supabase-js';
import { supabase } from '$lib/supabase';

export type SelfTestOutcome = {
	reached: number;
	pruned: number;
};

export async function sendTestNotification(title: string, body: string): Promise<SelfTestOutcome> {
	const { data: session } = await supabase().auth.getSession();
	if (!session.session) throw new Error('sign in first');

	const { data, error } = await supabase().functions.invoke<SelfTestOutcome>('notify-test', {
		body: { title, body }
	});
	if (error) throw new Error(await refusalOf(error));
	if (!data) throw new Error('the test notification returned nothing');
	return data;
}

async function refusalOf(failure: Error): Promise<string> {
	if (!(failure instanceof FunctionsHttpError)) return failure.message;
	const said = (await failure.context.json().catch(() => null)) as Record<string, unknown> | null;
	for (const field of ['error', 'msg', 'message']) {
		const told = said?.[field];
		if (typeof told === 'string' && told.trim()) return told;
	}
	return failure.message;
}
