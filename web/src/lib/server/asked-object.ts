export async function askedObject(request: Request): Promise<Record<string, unknown>> {
	const parsed = await request.json().catch(() => null);
	if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) return {};
	return parsed as Record<string, unknown>;
}
