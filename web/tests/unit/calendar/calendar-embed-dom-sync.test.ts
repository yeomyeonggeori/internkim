import { expect, test } from 'bun:test';

import { createCalendarDOMSyncScheduler } from '../../../src/routes/calendar/embed/calendar-embed-dom-sync';

test('runs only the latest scheduled DOM sync task', () => {
	const frameCallbacks: (() => void)[] = [];
	const timeoutCallbacks: (() => void)[] = [];
	const scheduleDOMSync = createCalendarDOMSyncScheduler({
		requestAnimationFrame: (callback) => {
			frameCallbacks.push(callback);
		},
		setTimeout: (callback) => {
			timeoutCallbacks.push(callback);
		}
	});
	const calls: string[] = [];

	scheduleDOMSync(() => calls.push('stale'));
	scheduleDOMSync(() => calls.push('latest'));
	runCallbacks(frameCallbacks);
	runCallbacks(timeoutCallbacks);

	expect(calls.length > 0).toBe(true);
	expect(calls.every((call) => call === 'latest')).toBe(true);
});

function runCallbacks(callbacks: (() => void)[]): void {
	for (let index = 0; index < callbacks.length; index += 1) {
		callbacks[index]();
	}
}
