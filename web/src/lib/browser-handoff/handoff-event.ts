import type { FieldBox } from './handoff-input';

export type HandoffOutcome = 'completed' | 'abandoned' | 'expired';

export type HandoffFrame = {
	kind: 'frame';
	handoffID: string;
	image: string;
	width: number;
	height: number;
	url: string;
	fields: FieldBox[];
};

export type HandoffEnded = {
	kind: 'ended';
	handoffID: string;
	outcome: HandoffOutcome;
};

export type HandoffTrouble = {
	kind: 'trouble';
	handoffID: string;
	reason: string;
};

export type HandoffEvent = HandoffFrame | HandoffEnded | HandoffTrouble;

const frameEventKind = 'browser.handoff.frame';
const endedEventKind = 'browser.handoff.ended';
const troubleEventKind = 'browser.handoff.trouble';
const outcomes: HandoffOutcome[] = ['completed', 'abandoned', 'expired'];

export function handoffEventOf(offered: unknown): HandoffEvent | null {
	if (typeof offered !== 'object' || offered === null) return null;
	const held = Object.fromEntries(Object.entries(offered));
	if (typeof held.handoffID !== 'string' || held.handoffID === '') return null;
	if (held.kind === frameEventKind) return frameOf(held.handoffID, held);
	if (held.kind === endedEventKind) return endedOf(held.handoffID, held);
	if (held.kind === troubleEventKind) return { kind: 'trouble', handoffID: held.handoffID, reason: typeof held.reason === 'string' ? held.reason : '' };
	return null;
}

function frameOf(handoffID: string, held: Record<string, unknown>): HandoffFrame | null {
	if (typeof held.image !== 'string' || held.image === '') return null;
	if (!isPositiveNumber(held.width) || !isPositiveNumber(held.height)) return null;
	const url = typeof held.url === 'string' ? held.url : '';
	return { kind: 'frame', handoffID, image: held.image, width: held.width, height: held.height, url, fields: fieldsOf(held.fields) };
}

function fieldsOf(offered: unknown): FieldBox[] {
	if (!Array.isArray(offered)) return [];
	return offered.flatMap((box: unknown) => {
		if (!Array.isArray(box) || box.length !== 4) return [];
		const [x, y, width, height]: unknown[] = box;
		if (!isFiniteNumber(x) || !isFiniteNumber(y) || !isPositiveNumber(width) || !isPositiveNumber(height)) return [];
		return [{ x, y, width, height }];
	});
}

function isFiniteNumber(offered: unknown): offered is number {
	return typeof offered === 'number' && Number.isFinite(offered);
}

function endedOf(handoffID: string, held: Record<string, unknown>): HandoffEnded | null {
	const outcome = outcomes.find((candidate) => candidate === held.outcome);
	return outcome ? { kind: 'ended', handoffID, outcome } : null;
}

function isPositiveNumber(offered: unknown): offered is number {
	return typeof offered === 'number' && Number.isFinite(offered) && offered > 0;
}
