import { describe, expect, test } from 'bun:test';
import { observeImageLoad, type ImageLoadState } from '../../../src/lib/components/image-loading';

class TestImage extends EventTarget {
	constructor(readonly complete: boolean, readonly naturalWidth: number) {
		super();
	}
}

describe('image request lifecycle', () => {
	test('a pending image stays pending until the native load event', () => {
		const image = new TestImage(false, 320);
		const states: ImageLoadState[] = [];
		const observation = observeImageLoad(image, value => states.push(value));
		expect(states).toEqual([]);
		image.dispatchEvent(new Event('load'));
		expect(states).toEqual(['loaded']);
		observation.destroy();
	});

	test('a cached image settles synchronously without a timer or another load event', () => {
		const image = new TestImage(true, 320);
		const states: ImageLoadState[] = [];
		const observation = observeImageLoad(image, value => states.push(value));
		expect(states).toEqual(['loaded']);
		observation.destroy();
	});

	test('cached failures and network errors end loading', () => {
		const states: ImageLoadState[] = [];
		const cachedFailure = observeImageLoad(new TestImage(true, 0), value => states.push(value));
		const image = new TestImage(false, 0);
		const networkFailure = observeImageLoad(image, value => states.push(value));
		image.dispatchEvent(new Event('error'));
		expect(states).toEqual(['error', 'error']);
		cachedFailure.destroy();
		networkFailure.destroy();
	});

	test('removed or replaced images cannot publish stale events', () => {
		const image = new TestImage(false, 320);
		const states: ImageLoadState[] = [];
		const observation = observeImageLoad(image, value => states.push(value));
		observation.destroy();
		image.dispatchEvent(new Event('load'));
		image.dispatchEvent(new Event('error'));
		expect(states).toEqual([]);
	});
});

test('the mounted image component clears old results, handles cached pixels, and cleans up replaced nodes', async () => {
	const fixture = new URL('./image-loading.fixture.ts', import.meta.url).pathname;
	const run = Bun.spawn([process.execPath, '--conditions', 'browser', fixture], { stdout: 'pipe', stderr: 'pipe' });
	const [output, errors, exitCode] = await Promise.all([
		new Response(run.stdout).text(), new Response(run.stderr).text(), run.exited
	]);
	expect(exitCode, errors).toBe(0);
	expect(JSON.parse(output)).toEqual({
		cached: 'loaded', cachedSkeletons: 0, replacementPending: 'loading', staleResultIgnored: 'loading',
		replacementLoaded: 'loaded', failed: 'error', fallback: 'Sample picture: Image could not be loaded', removed: null,
		sizing: [
			{ name: 'known portrait', pendingAspectRatio: '240 / 320', loadedAspectRatio: '240 / 320' },
			...['missing', 'partial', 'zero', 'negative', 'infinite', 'not a number'].map(name => ({
				name, pendingAspectRatio: '320 / 320', loadedAspectRatio: '240 / 320'
			}))
		]
	});
});
