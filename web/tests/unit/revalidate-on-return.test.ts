import { describe, expect, test } from 'bun:test';
import { revalidateOnReturn } from '../../src/lib/revalidate-on-return';

class TestDocument extends EventTarget {
	visibilityState: DocumentVisibilityState = 'visible';
	focused = true;
	hasFocus() { return this.focused; }
}

function browserEvents() {
	return { windowEvents: new EventTarget(), documentEvents: new TestDocument() };
}

describe('session validation when returning to the app', () => {
	test('coalesces visible then focus, and focus then visible', () => {
		for (const order of [['visibilitychange', 'focus'], ['focus', 'visibilitychange']]) {
			const { windowEvents, documentEvents } = browserEvents();
			let validations = 0;
			const stop = revalidateOnReturn(windowEvents, documentEvents, () => { validations += 1; });
			for (const event of order) {
				(event === 'focus' ? windowEvents : documentEvents).dispatchEvent(new Event(event));
			}
			expect(validations).toBe(1);
			stop();
		}
	});

	test('every new blur/focus validates even while a previous request is unresolved', () => {
		const { windowEvents, documentEvents } = browserEvents();
		let validations = 0;
		const stop = revalidateOnReturn(windowEvents, documentEvents, () => { validations += 1; });
		windowEvents.dispatchEvent(new Event('focus'));
		windowEvents.dispatchEvent(new Event('blur'));
		windowEvents.dispatchEvent(new Event('focus'));
		expect(validations).toBe(2);
		stop();
	});

	test('a hidden tab waits, and each later return validates again', () => {
		const { windowEvents, documentEvents } = browserEvents();
		let validations = 0;
		const stop = revalidateOnReturn(windowEvents, documentEvents, () => { validations += 1; });
		for (let cycle = 0; cycle < 2; cycle += 1) {
			documentEvents.visibilityState = 'hidden';
			documentEvents.dispatchEvent(new Event('visibilitychange'));
			windowEvents.dispatchEvent(new Event('focus'));
			expect(validations).toBe(cycle);
			documentEvents.visibilityState = 'visible';
			documentEvents.dispatchEvent(new Event('visibilitychange'));
			windowEvents.dispatchEvent(new Event('focus'));
			expect(validations).toBe(cycle + 1);
		}
		stop();
	});

	test('a visible unfocused window still validates on later focus', () => {
		const { windowEvents, documentEvents } = browserEvents();
		let validations = 0;
		const stop = revalidateOnReturn(windowEvents, documentEvents, () => { validations += 1; });
		documentEvents.focused = false;
		documentEvents.dispatchEvent(new Event('visibilitychange'));
		documentEvents.focused = true;
		windowEvents.dispatchEvent(new Event('focus'));
		expect(validations).toBe(2);
		stop();
	});

	test('removes all listeners when the shell is destroyed', () => {
		const { windowEvents, documentEvents } = browserEvents();
		let validations = 0;
		const stop = revalidateOnReturn(windowEvents, documentEvents, () => { validations += 1; });
		stop();
		windowEvents.dispatchEvent(new Event('blur'));
		windowEvents.dispatchEvent(new Event('focus'));
		documentEvents.dispatchEvent(new Event('visibilitychange'));
		expect(validations).toBe(0);
	});
});
