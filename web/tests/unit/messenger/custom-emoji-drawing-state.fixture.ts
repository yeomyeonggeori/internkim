import { mock } from 'bun:test';

Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
mock.module('$lib/supabase', () => ({ isSupabaseConfigured: () => true }));
mock.module('$lib/messenger/cache-scope', () => ({ onMessengerCacheReset: () => () => {} }));
mock.module('$lib/messenger/messenger-api', () => ({
	fetchCustomEmojiNames: () => Promise.resolve(['sample_drawn', 'sample_broken']),
	fetchCustomEmojiImage: (name: string) => Promise.resolve(name === 'sample_drawn' ? { dataURL: 'drawn-image' } : null)
}));

const { customEmoji } = await import('$lib/stores/custom-emoji.svelte');
const beforeTheListArrives = customEmoji.isDrawing('sample_drawn');
await customEmoji.load();
const listedButNotYetDrawn = customEmoji.isDrawing('sample_drawn');
const notInTheList = customEmoji.isDrawing('sample_unlisted');
await customEmoji.draw(['sample_drawn', 'sample_broken']);
console.log(JSON.stringify({
	beforeTheListArrives,
	listedButNotYetDrawn,
	notInTheList,
	drawn: customEmoji.isDrawing('sample_drawn'),
	couldNotBeDrawn: customEmoji.isDrawing('sample_broken')
}));
