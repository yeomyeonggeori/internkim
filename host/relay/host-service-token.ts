import { timingSafeEqual } from 'node:crypto';
import { reasonOf } from './failure';

export function hostServiceTokenPath(): string | undefined {
	return process.env.HOST_SERVICE_TOKEN_PATH?.trim() || undefined;
}

export async function asAHostService(init: RequestInit, tokenPath = hostServiceTokenPath()): Promise<RequestInit> {
	if (!tokenPath) return init;
	const headers = new Headers(init.headers);
	headers.set('Authorization', `Bearer ${await keptToken(tokenPath)}`);
	return { ...init, headers };
}

export async function carriesTheHostServiceToken(request: Request, tokenPath = hostServiceTokenPath()): Promise<boolean> {
	if (!tokenPath) return true;
	const expected = await keptToken(tokenPath).catch(() => '');
	const authorization = request.headers.get('Authorization') ?? '';
	if (!expected || !authorization.startsWith('Bearer ')) return false;
	const offered = Buffer.from(authorization.slice('Bearer '.length));
	const kept = Buffer.from(expected);
	return offered.length === kept.length && timingSafeEqual(offered, kept);
}

async function keptToken(tokenPath: string): Promise<string> {
	const kept = await Bun.file(tokenPath)
		.text()
		.catch((failure: unknown) => {
			throw new Error(`the token the host services share could not be read at ${tokenPath}: ${reasonOf(failure)}`);
		});
	return kept.trim();
}
