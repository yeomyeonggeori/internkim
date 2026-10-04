import { supabase } from '$lib/supabase';
import { resultOrRefusal, ToolRefused } from '$lib/tool-answer';

export { ToolRefused };

export function isRefusalCode(refusal: unknown, errorCode: string): boolean {
	return refusal instanceof ToolRefused && refusal.errorCode === errorCode;
}

export async function memberAccessToken(): Promise<string> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');
	return accessToken;
}

export async function invokeTool<Result>(
	name: string,
	input: Record<string, unknown>
): Promise<Result> {
	const accessToken = await memberAccessToken();

	const response = await fetch(`/api/v1/tools/${name}/invoke`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ input })
	});
	return resultOrRefusal(name, response.status, await response.json().catch(() => null)) as Result;
}
