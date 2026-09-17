export type HandoffOutcome = 'completed' | 'abandoned' | 'expired';

export type HandoffFrame = {
	kind: 'frame';
	handoffID: string;
	image: string;
	width: number;
	height: number;
	url: string;
};

export type HandoffEnded = {
	kind: 'ended';
	handoffID: string;
	outcome: HandoffOutcome;
};

export type HandoffEvent = HandoffFrame | HandoffEnded;

const frameEventKind = 'browser.handoff.frame';
const endedEventKind = 'browser.handoff.ended';
const outcomes: HandoffOutcome[] = ['completed', 'abandoned', 'expired'];

export function handoffEventOf(offered: unknown): HandoffEvent | null {
	if (typeof offered !== 'object' || offered === null) return null;
	const held = Object.fromEntries(Object.entries(offered));
	if (typeof held.handoffID !== 'string' || held.handoffID === '') return null;
	if (held.kind === frameEventKind) return frameOf(held.handoffID, held);
	if (held.kind === endedEventKind) return endedOf(held.handoffID, held);
	return null;
}

function frameOf(handoffID: string, held: Record<string, unknown>): HandoffFrame | null {
	if (typeof held.image !== 'string' || held.image === '') return null;
	if (!isPositiveNumber(held.width) || !isPositiveNumber(held.height)) return null;
	const url = typeof held.url === 'string' ? held.url : '';
	return { kind: 'frame', handoffID, image: held.image, width: held.width, height: held.height, url };
}

function endedOf(handoffID: string, held: Record<string, unknown>): HandoffEnded | null {
	const outcome = outcomes.find((candidate) => candidate === held.outcome);
	return outcome ? { kind: 'ended', handoffID, outcome } : null;
}

function isPositiveNumber(offered: unknown): offered is number {
	return typeof offered === 'number' && Number.isFinite(offered) && offered > 0;
}
