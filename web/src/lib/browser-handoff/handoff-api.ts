import { callCompanyApp } from '$lib/host-bridge';
import type { HandoffInput, Viewport } from './handoff-input';

export type HandoffWatch = {
	handoffID: string;
	message: string;
	expiresAt: string;
	viewport: Viewport;
};

export type WatchAnswer =
	| { state: 'watching'; watch: HandoffWatch }
	| { state: 'missing' }
	| { state: 'refused' }
	| { state: 'failed'; reason: string };

export type FinishingOutcome = 'completed' | 'abandoned';

export async function watchHandoff(handoffID: string, viewport: Viewport | null): Promise<WatchAnswer> {
	const answer = await callCompanyApp({
		capability: 'person.browser.handoff.watch',
		body: { handoffID, ...(viewport ? { viewport } : {}) }
	});
	if (answer.status === 404) return { state: 'missing' };
	if (answer.status === 403) return { state: 'refused' };
	if (answer.status !== 200) return { state: 'failed', reason: refusalReasonOf(answer.status, answer.body) };
	const watch = readHandoffWatch(answer.body);
	if (!watch) throw new Error(`watching the browser handoff returned ${answer.status}`);
	return { state: 'watching', watch };
}

export async function sendHandoffInputs(handoffID: string, inputs: HandoffInput[]): Promise<void> {
	const answer = await callCompanyApp({ capability: 'person.browser.handoff.input', body: { handoffID, inputs } });
	if (answer.status >= 400) throw new Error(refusalReasonOf(answer.status, answer.body));
}

export async function finishHandoff(handoffID: string, outcome: FinishingOutcome): Promise<void> {
	const answer = await callCompanyApp({ capability: 'person.browser.handoff.finish', body: { handoffID, outcome } });
	if (answer.status >= 400 && answer.status !== 404) throw new Error(`finishing the browser handoff returned ${answer.status}`);
}

export function readHandoffWatch(offered: unknown): HandoffWatch | null {
	const held = recordOf(offered);
	const viewport = recordOf(held.viewport);
	if (typeof held.handoffID !== 'string' || typeof held.expiresAt !== 'string') return null;
	if (!isPositiveNumber(viewport.width) || !isPositiveNumber(viewport.height)) return null;
	return {
		handoffID: held.handoffID,
		message: typeof held.message === 'string' ? held.message : '',
		expiresAt: held.expiresAt,
		viewport: { width: viewport.width, height: viewport.height }
	};
}

function refusalReasonOf(status: number, body: unknown): string {
	const error = recordOf(body).error;
	return typeof error === 'string' && error !== '' ? error : `the device answered ${status}`;
}

function recordOf(offered: unknown): Record<string, unknown> {
	if (typeof offered !== 'object' || offered === null) return {};
	return Object.fromEntries(Object.entries(offered));
}

function isPositiveNumber(offered: unknown): offered is number {
	return typeof offered === 'number' && Number.isFinite(offered) && offered > 0;
}
