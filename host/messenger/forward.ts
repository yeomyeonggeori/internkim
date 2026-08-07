export type ActorCredential = { kind: string; secret: string };

export type Call = {
	callID?: string;
	capability?: string;
	replyTo?: string;
	body?: Record<string, unknown>;
};

export type Answer = { callID: string; status: number; body: unknown };

const personPrefix = 'person.';

export function isPersonCapability(capability: string): boolean {
	return capability.startsWith(personPrefix);
}

export function reportableTopic(call: Call): string | null {
	const offered = call.replyTo;
	if (typeof offered !== 'string') return null;
	return /^[0-9a-f-]{36}$/i.test(offered) ? offered : null;
}

export function actorOf(call: Call): ActorCredential | null {
	const offered = call.body?.actor;
	if (typeof offered !== 'object' || offered === null) return null;
	const { kind, secret } = offered as { kind?: unknown; secret?: unknown };
	if (typeof kind !== 'string' || typeof secret !== 'string') return null;
	if (!kind.trim() || !secret.trim()) return null;
	return { kind, secret };
}

export async function forwardToChatd(
	chatdBaseURL: string,
	platform: string,
	capability: string,
	body: Record<string, unknown>
): Promise<{ status: number; body: unknown }> {
	const response = await fetch(
		`${chatdBaseURL}/v1/platform/${encodeURIComponent(platform)}/${encodeURIComponent(capability)}`,
		{
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body)
		}
	);
	return { status: response.status, body: await response.json().catch(() => null) };
}
