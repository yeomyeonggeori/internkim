import { supabase } from '$lib/supabase';

export class ToolRefused extends Error {
	constructor(
		message: string,
		readonly errorCode: string | undefined,
		readonly status: number
	) {
		super(message);
		this.name = 'ToolRefused';
	}
}

export function isRefusalCode(refusal: unknown, errorCode: string): boolean {
	return refusal instanceof ToolRefused && refusal.errorCode === errorCode;
}

export async function invokeTool<Result>(
	name: string,
	input: Record<string, unknown>
): Promise<Result> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');

	const response = await fetch(`/api/v1/tools/${name}/invoke`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ input })
	});
	const answered = (await response.json().catch(() => null)) as
		| { result?: Result; error?: string; errorCode?: string }
		| null;
	if (!response.ok) {
		throw new ToolRefused(
			answered?.error ?? `${name} answered ${response.status}`,
			answered?.errorCode,
			response.status
		);
	}
	if (!answered || answered.result === undefined) throw new Error(`${name} answered nothing`);
	return answered.result;
}
