import type { Environment } from './agent-request';
import type { CompanyCallTransport } from './public-api/company-call';

export type FunctionAnswer = { status: number; body: unknown };

const askThroughTheRuntime: CompanyCallTransport = (url, options) => fetch(url, options);

export function projectFunctionURL(environment: Environment, name: string): string {
	const projectURL = (environment.SUPABASE_URL ?? '').replace(/\/+$/, '');
	return projectURL ? `${projectURL}/functions/v1/${name}` : '';
}

export function planeKeyOf(environment: Environment): string {
	return environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
}

export async function askTheProject(
	environment: Environment,
	name: string,
	body: unknown,
	bearer?: string,
	transport: CompanyCallTransport = askThroughTheRuntime
): Promise<FunctionAnswer> {
	const url = projectFunctionURL(environment, name);
	const key = bearer ?? planeKeyOf(environment);
	if (!url || !key) return { status: 503, body: { error: 'the project is not configured' } };

	const response = await transport(url, {
		method: 'POST',
		headers: { Authorization: `Bearer ${key}`, 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	return { status: response.status, body: await response.json().catch(() => null) };
}
