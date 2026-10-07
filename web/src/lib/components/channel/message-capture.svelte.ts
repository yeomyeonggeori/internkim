import { rangeAfterClick, type MessageRange } from './message-range';

export type MessageCapture = ReturnType<typeof createMessageCapture>;

export function createMessageCapture(orderedMessageIDs: () => string[]) {
	let isCapturing = $state(false);
	let startID = $state('');
	let endID = $state('');

	const range = $derived.by((): MessageRange | null => {
		const order = orderedMessageIDs();
		const start = order.indexOf(startID);
		const end = order.indexOf(endID);
		if (start === -1 || end === -1) return null;
		return { start, end };
	});

	const chosenIDs = $derived(range === null ? [] : orderedMessageIDs().slice(range.start, range.end + 1));

	function choose(messageID: string): void {
		const order = orderedMessageIDs();
		const clicked = order.indexOf(messageID);
		if (clicked === -1) return;
		const next = rangeAfterClick(range, clicked);
		startID = order[next.start];
		endID = order[next.end];
	}

	function clearChoice(): void {
		startID = '';
		endID = '';
	}

	return {
		get isCapturing() {
			return isCapturing;
		},
		get chosenIDs() {
			return chosenIDs;
		},
		get firstID() {
			return chosenIDs[0] ?? '';
		},
		get lastID() {
			return chosenIDs.at(-1) ?? '';
		},
		begin(): void {
			clearChoice();
			isCapturing = true;
		},
		end(): void {
			clearChoice();
			isCapturing = false;
		},
		choose,
		clearChoice
	};
}
