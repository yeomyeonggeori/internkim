import { afterEach, describe, expect, test } from 'bun:test';
import { keepAppInVisualViewport } from '../../src/lib/app-viewport';

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

function installPhone(): { viewport: FakeViewport; root: FakeRoot; stop: () => void } {
	const viewport = new FakeViewport();
	const root = new FakeRoot();
	const fakeWindow = Object.assign(new EventTarget(), {
		innerWidth: 390,
		innerHeight: 844,
		visualViewport: viewport,
		scrollY: 0,
		scrollTo(_left: number, top: number) {
			fakeWindow.scrollY = top;
		}
	});
	Object.assign(globalThis, { window: fakeWindow, document: { documentElement: root } });
	return { viewport, root, stop: keepAppInVisualViewport() };
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
});
