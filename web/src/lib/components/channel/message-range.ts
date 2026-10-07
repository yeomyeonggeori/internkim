export type MessageRange = { start: number; end: number };

export function rangeAfterClick(range: MessageRange | null, clicked: number): MessageRange {
	if (range === null) return { start: clicked, end: clicked };
	if (clicked < range.start) return { start: clicked, end: range.end };
	if (clicked > range.end) return { start: range.start, end: clicked };
	const distanceFromStart = clicked - range.start;
	const distanceFromEnd = range.end - clicked;
	if (distanceFromStart < distanceFromEnd) return { start: clicked, end: range.end };
	return { start: range.start, end: clicked };
}
