import { mock } from 'bun:test';

type ImageReply = { dataURL: string } | null;
function gate() {
	let release: (value: ImageReply) => void = () => {};
	const promise = new Promise<ImageReply>(resolve => { release = resolve; });
	return { promise, release };
}

const concurrent = gate();
const oldScope = gate();
const newScope = gate();
let retryCalls = 0;
let concurrentCalls = 0;
let scopeCalls = 0;
let reset = () => {};
Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
mock.module('$lib/supabase', () => ({ isSupabaseConfigured: () => true }));
mock.module('$lib/messenger/cache-scope', () => ({ onMessengerCacheReset: (callback: () => void) => { reset = callback; return () => {}; } }));
mock.module('$lib/messenger/messenger-api', () => ({
	fetchCustomEmojiNames: () => Promise.resolve(['sample_retry', 'sample_concurrent', 'sample_scope']),
	fetchCustomEmojiImage: (name: string): Promise<ImageReply> => {
		if (name === 'sample_retry') {
			retryCalls += 1;
			return retryCalls === 1 ? Promise.reject(new Error('Temporary image failure')) : Promise.resolve({ dataURL: 'retry-image' });
		}
		if (name === 'sample_concurrent') { concurrentCalls += 1; return concurrent.promise; }
		if (name === 'sample_scope') { scopeCalls += 1; return scopeCalls === 1 ? oldScope.promise : newScope.promise; }
		return Promise.resolve(null);
	}
}));

const { customEmoji } = await import('$lib/stores/custom-emoji.svelte');
await customEmoji.load();
await customEmoji.draw(['sample_retry']);
const failedImageWasNotStored = !customEmoji.nameToURL.has('sample_retry');
await customEmoji.draw(['sample_retry']);
await customEmoji.draw(['sample_retry']);
const retriedImage = customEmoji.nameToURL.get('sample_retry');
const first = customEmoji.draw(['sample_concurrent']);
const second = customEmoji.draw(['sample_concurrent']);
const inFlightCalls = concurrentCalls;
concurrent.release({ dataURL: 'concurrent-image' });
await Promise.all([first, second]);
const oldDrawing = customEmoji.draw(['sample_scope']);
reset();
await customEmoji.load();
const newDrawing = customEmoji.draw(['sample_scope']);
oldScope.release(null);
await oldDrawing;
const joinedNewDrawing = customEmoji.draw(['sample_scope']);
const requestsAfterOldFailure = scopeCalls;
newScope.release({ dataURL: 'new-scope-image' });
await Promise.all([newDrawing, joinedNewDrawing]);
console.log(JSON.stringify({ failedImageWasNotStored, retriedImage, retryCalls, inFlightCalls, requestsAfterOldFailure, scopedImage: customEmoji.nameToURL.get('sample_scope') }));
