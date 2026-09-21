const returnParameter = 'return';

// WHATWG URL §4.4: a special scheme reads "\" as "/", so "/\evil.test" resolves
// off this origin and a prefix test does not see it.
const nowhere = 'https://internkim.invalid';

export function ownPath(offered: unknown): string {
	if (typeof offered !== 'string' || !offered.startsWith('/')) return '';
	const resolved = new URL(offered, nowhere);
	if (resolved.origin !== nowhere) return '';
	return resolved.pathname + resolved.search + resolved.hash;
}

export function returnPathOf(url: URL): string {
	return ownPath(url.searchParams.get(returnParameter));
}

export function withReturnPath(destination: string, returnPath: string): string {
	const kept = ownPath(returnPath);
	if (!kept) return destination;
	const separator = destination.includes('?') ? '&' : '?';
	return `${destination}${separator}${returnParameter}=${encodeURIComponent(kept)}`;
}
