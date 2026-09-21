const returnParameter = 'return';

// WHATWG URL §4.4: a special scheme reads "\" as "/", so "/\evil.test" resolves
// off this origin and a prefix test does not see it.
const nowhere = 'https://internkim.invalid';

export function ownPath(offered: unknown): string {
	if (typeof offered !== 'string' || !offered.startsWith('/')) return '';
	const resolved = resolvedAgainstNowhere(offered);
	if (!resolved || resolved.origin !== nowhere) return '';
	return resolved.pathname + resolved.search + resolved.hash;
}

export function returnPathOf(url: URL): string {
	return ownPath(url.searchParams.get(returnParameter));
}

export function withReturnPath(destination: string, returnPath: string): string {
	const kept = ownPath(returnPath);
	if (!kept) return destination;
	const fragmentAt = destination.indexOf('#');
	const asked = fragmentAt === -1 ? destination : destination.slice(0, fragmentAt);
	const fragment = fragmentAt === -1 ? '' : destination.slice(fragmentAt);
	const separator = asked.includes('?') ? '&' : '?';
	return `${asked}${separator}${returnParameter}=${encodeURIComponent(kept)}${fragment}`;
}

// A candidate that is not a URL at all, such as the unterminated IPv6 literal
// in "//[", makes the WHATWG parser throw where every caller needs an answer.
function resolvedAgainstNowhere(offered: string): URL | null {
	try {
		return new URL(offered, nowhere);
	} catch {
		return null;
	}
}
