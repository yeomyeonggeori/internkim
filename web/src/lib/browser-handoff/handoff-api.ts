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
	| { state: 'refused' };

export type FinishingOutcome = 'completed' | 'abandoned';

export async function watchHandoff(handoffID: string, viewport: Viewport | null): Promise<WatchAnswer> {
	const answer = await callCompanyApp({
		capability: 'person.browser.handoff.watch',
		body: { handoffID, ...(viewport ? { viewport } : {}) }
	});
	if (answer.status === 404) return { state: 'missing' };
	if (answer.status === 403) return { state: 'refused' };
	const watch = answer.status === 200 ? readHandoffWatch(answer.body) : null;
	if (!watch) throw new Error(`watching the browser handoff returned ${answer.status}`);
	return { state: 'watching', watch };
}

export async function sendHandoffInputs(handoffID: string, inputs: HandoffInput[]): Promise<void> {
	const answer = await callCompanyApp({ capability: 'person.browser.handoff.input', body: { handoffID, inputs } });
	if (answer.status >= 400) throw new Error(`the browser handoff refused its input with ${answer.status}`);
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

function recordOf(offered: unknown): Record<string, unknown> {
	if (typeof offered !== 'object' || offered === null) return {};
	return Object.fromEntries(Object.entries(offered));
}

function isPositiveNumber(offered: unknown): offered is number {
	return typeof offered === 'number' && Number.isFinite(offered) && offered > 0;
}
