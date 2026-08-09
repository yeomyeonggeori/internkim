const linkPattern = /https?:\/\/[^\s<>"'`)\]]+/;

export function firstLinkIn(text: string): string {
	const found = text.match(linkPattern)?.[0] ?? '';
	return found.replace(/[.,;:!?]+$/, '');
}
