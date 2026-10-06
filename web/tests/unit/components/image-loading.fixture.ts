import { mock } from 'bun:test';
import { readFileSync } from 'node:fs';
import { compile } from 'svelte/compiler';
import { Window } from 'happy-dom';

const browserWindow = new Window();
for (const name of Object.getOwnPropertyNames(browserWindow)) {
	if (name in globalThis) continue;
	Object.defineProperty(globalThis, name, { value: Reflect.get(browserWindow, name), configurable: true, writable: true });
}
Object.assign(globalThis, { window: browserWindow, document: browserWindow.document, Event: browserWindow.Event });
const decodedSources = new Set(['/cached.svg']);
Object.defineProperties(browserWindow.HTMLImageElement.prototype, {
	complete: { configurable: true, get(this: HTMLImageElement) { return decodedSources.has(this.getAttribute('src') ?? ''); } },
	naturalWidth: { configurable: true, get(this: HTMLImageElement) { return decodedSources.has(this.getAttribute('src') ?? '') ? 240 : 0; } },
	naturalHeight: { configurable: true, get(this: HTMLImageElement) { return decodedSources.has(this.getAttribute('src') ?? '') ? 320 : 0; } }
});

mock.module('../../../src/lib/i18n/page-text.svelte', () => ({ createPageText: () => ({ loading: 'Loading image', failed: 'Image could not be loaded' }) }));
Bun.plugin({ name: 'image-component-fixture', setup(build) {
	build.onLoad({ filter: /\.svelte$/ }, ({ path }) => ({
		contents: compile(readFileSync(path, 'utf8'), { filename: path, generate: 'client' }).js.code,
		loader: 'js'
	}));
} });

const { flushSync, mount, unmount } = await import('svelte');
const { default: Harness } = await import('./image-loading-harness.fixture.svelte');
const target = document.createElement('div');
document.body.appendChild(target);
const instance = mount(Harness, { target });
flushSync();
const status = () => target.querySelector('[data-loading-image]')?.getAttribute('data-loading-image') ?? null;
const image = () => {
	const found = target.querySelector('img');
	if (!found) throw new Error('The image did not mount');
	return found;
};
const cached = status();
const cachedSkeletons = target.querySelectorAll('[data-slot="skeleton"]').length;
const oldImage = image();
instance.replace('/replacement.svg');
flushSync();
const replacementPending = status();
const replacedImage = image();
oldImage.dispatchEvent(new Event('load'));
oldImage.dispatchEvent(new Event('error'));
flushSync();
const staleResultIgnored = status();
decodedSources.add('/replacement.svg');
replacedImage.dispatchEvent(new Event('load'));
flushSync();
const replacementLoaded = status();
instance.replace('/failed.svg');
flushSync();
image().dispatchEvent(new Event('error'));
flushSync();
const failed = status();
const fallback = target.querySelector('[role="img"]')?.getAttribute('aria-label');
instance.replace('/late.svg');
flushSync();
const lateImage = image();
instance.remove();
flushSync();
lateImage.dispatchEvent(new Event('load'));
lateImage.dispatchEvent(new Event('error'));
flushSync();
const removed = status();
await unmount(instance);

const { default: LoadingImage } = await import('../../../src/lib/components/loading-image.svelte');
const sizingCases: { name: string; width?: number; height?: number }[] = [
	{ name: 'known portrait', width: 240, height: 320 },
	{ name: 'missing' },
	{ name: 'partial', width: 240 },
	{ name: 'zero', width: 0, height: 320 },
	{ name: 'negative', width: 240, height: -320 },
	{ name: 'infinite', width: Number.POSITIVE_INFINITY, height: 320 },
	{ name: 'not a number', width: 240, height: Number.NaN }
];
const sizing = [];
for (const entry of sizingCases) {
	const source = `/sizing-${entry.name}.svg`;
	const component = mount(LoadingImage, { target, props: { src: source, alt: 'Sizing sample', width: entry.width, height: entry.height, loading: 'eager' } });
	flushSync();
	const frame = target.querySelector<HTMLSpanElement>('[data-loading-image]');
	if (!frame) throw new Error('The sizing frame did not mount');
	const pendingAspectRatio = frame.style.aspectRatio;
	decodedSources.add(source);
	image().dispatchEvent(new Event('load'));
	flushSync();
	sizing.push({ name: entry.name, pendingAspectRatio, loadedAspectRatio: frame.style.aspectRatio });
	await unmount(component);
}
console.log(JSON.stringify({ cached, cachedSkeletons, replacementPending, staleResultIgnored, replacementLoaded, failed, fallback, removed, sizing }));
