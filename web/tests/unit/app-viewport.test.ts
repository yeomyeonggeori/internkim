import { afterEach, describe, expect, test } from 'bun:test';
import { keepAppInVisualViewport } from '../../src/lib/app-viewport';
import type { NativeKeyboardWatcher } from '../../src/lib/native-shell/keyboard';

class FakeViewport extends EventTarget {
	height = 844;
	offsetTop = 0;
	scale = 1;
}

class FakeRoot {
	properties = new Map<string, string>();
	attributes = new Set<string>();
	style = {
		setProperty: (name: string, value: string) => this.properties.set(name, value),
		removeProperty: (name: string) => this.properties.delete(name)
	};
	setAttribute(name: string): void {
		this.attributes.add(name);
	}
	removeAttribute(name: string): void {
		this.attributes.delete(name);
	}
}

const originalWindow = globalThis.window;
const originalDocument = globalThis.document;

const composer = { matches: (selector: string) => selector.includes('contenteditable') };

function installPhone(watchNativeKeyboard?: NativeKeyboardWatcher): { viewport: FakeViewport; root: FakeRoot; fakeDocument: { activeElement: typeof composer | null }; stop: () => void } {
	const viewport = new FakeViewport();
	const root = new FakeRoot();
	const fakeDocument: { documentElement: FakeRoot; activeElement: typeof composer | null } = { documentElement: root, activeElement: composer };
	const fakeWindow = Object.assign(new EventTarget(), {
		innerWidth: 390,
		innerHeight: 844,
		visualViewport: viewport,
		scrollY: 0,
		scrollTo(_left: number, top: number) {
			fakeWindow.scrollY = top;
		}
	});
	Object.assign(globalThis, { window: fakeWindow, document: fakeDocument });
	return { viewport, root, fakeDocument, stop: keepAppInVisualViewport(watchNativeKeyboard) };
}

afterEach(() => {
	Object.assign(globalThis, { window: originalWindow, document: originalDocument });
});

describe('keepAppInVisualViewport', () => {
	test('leaves the layout to the browser while only the toolbar moves', () => {
		const { viewport, root, stop } = installPhone();
		viewport.height = 790;
		viewport.dispatchEvent(new Event('resize'));
		viewport.offsetTop = 12;
		viewport.dispatchEvent(new Event('scroll'));
		expect([...root.properties.keys()]).toEqual([]);
		expect(root.attributes.size).toBe(0);
		stop();
	});

	test('follows the visual viewport while the keyboard is open and lets go when it closes', () => {
		const { viewport, root, stop } = installPhone();
		viewport.height = 500;
		viewport.dispatchEvent(new Event('resize'));
		expect(root.properties.get('--app-viewport-height')).toBe('500px');
		expect(root.properties.get('--app-viewport-bottom')).toBe('344px');
		expect(root.attributes.has('data-mobile-keyboard-open')).toBe(true);
		window.scrollY = 160;
		viewport.height = 844;
		viewport.dispatchEvent(new Event('resize'));
		expect([...root.properties.keys()]).toEqual([]);
		expect(root.attributes.size).toBe(0);
		expect(window.scrollY).toBe(0);
		stop();
	});

	test('keeps the keyboard open when WebKit briefly reports the window as tall as the keyboard leaves it', () => {
		const { viewport, root, stop } = installPhone();
		viewport.height = 448;
		viewport.dispatchEvent(new Event('resize'));
		window.innerHeight = 448;
		viewport.dispatchEvent(new Event('scroll'));
		expect(root.attributes.has('data-mobile-keyboard-open')).toBe(true);
		window.innerHeight = 844;
		viewport.height = 844;
		viewport.dispatchEvent(new Event('resize'));
		expect(root.attributes.size).toBe(0);
		stop();
	});

	test('notices the keyboard when Safari shrinks the window and scrolls the page above it', () => {
		const { viewport, root, stop } = installPhone();
		window.innerHeight = 448;
		window.scrollY = 396;
		viewport.height = 448;
		viewport.offsetTop = 396;
		viewport.dispatchEvent(new Event('resize'));
		expect(root.attributes.has('data-mobile-keyboard-open')).toBe(true);
		expect(root.properties.get('--app-viewport-top')).toBe('396px');
		expect(root.properties.get('--app-viewport-bottom')).toBe('0px');
		stop();
	});

	test('leaves the chrome alone when the window gets shorter while nobody is typing', () => {
		const { viewport, root, fakeDocument, stop } = installPhone();
		fakeDocument.activeElement = null;
		window.innerHeight = 500;
		viewport.height = 500;
		viewport.dispatchEvent(new Event('resize'));
		expect(root.attributes.size).toBe(0);
		stop();
	});

	test('hides the phone chrome while the native shell says its keyboard is open, even when the web view shrinks with it', () => {
		let tellKeyboard: (isOpen: boolean) => void = () => {};
		let isWatching = false;
		const { root, stop } = installPhone((onChange) => {
			tellKeyboard = onChange;
			isWatching = true;
			return () => {
				isWatching = false;
			};
		});
		tellKeyboard(true);
		expect(root.attributes.has('data-mobile-keyboard-open')).toBe(true);
		expect(root.properties.get('--app-viewport-safe-bottom')).toBe('0px');
		expect(root.properties.has('--app-viewport-bottom')).toBe(false);
		window.dispatchEvent(new Event('resize'));
		expect(root.attributes.has('data-mobile-keyboard-open')).toBe(true);
		tellKeyboard(false);
		expect([...root.properties.keys()]).toEqual([]);
		expect(root.attributes.size).toBe(0);
		stop();
		expect(isWatching).toBe(false);
	});
});
