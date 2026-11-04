const linkedMessageParameter = 'message';

export function linkedMessageIDOf(url: URL): string | undefined {
	return url.searchParams.get(linkedMessageParameter) || undefined;
}

export function withoutLinkedMessage(url: URL): URL {
	const kept = new URL(url);
	kept.searchParams.delete(linkedMessageParameter);
	return kept;
}
