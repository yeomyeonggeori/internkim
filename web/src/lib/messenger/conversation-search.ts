export type SearchExcerpt = { before: string; match: string; after: string };

const excerptContextCharacters = 24;
const ellipsis = '…';

export function searchExcerpt(text: string, query: string): SearchExcerpt {
	const flattened = text.replace(/\s+/g, ' ').trim();
	const wanted = query.replace(/\s+/g, ' ').trim();
	const start = wanted === '' ? -1 : flattened.toLowerCase().indexOf(wanted.toLowerCase());
	if (start < 0) return { before: '', match: '', after: flattened };
	const end = start + wanted.length;
	const beforeStart = Math.max(0, start - excerptContextCharacters);
	return {
		before: (beforeStart > 0 ? ellipsis : '') + flattened.slice(beforeStart, start),
		match: flattened.slice(start, end),
		after: flattened.slice(end)
	};
}

export function searchMatchTime(isoTimestamp: string): string {
	return new Date(isoTimestamp).toLocaleString('ko-KR', {
		month: 'long',
		day: 'numeric',
		hour: 'numeric',
		minute: '2-digit',
		hour12: true
	});
}
