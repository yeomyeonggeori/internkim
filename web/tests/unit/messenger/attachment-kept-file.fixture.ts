import { mock } from 'bun:test';

Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
const nextTurn = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
const origin = 'https://workspace.example.test';
const keptAddress = 'https://messenger.example.test/files/kept-picture';
const newAddress = 'https://messenger.example.test/files/new-picture';
const documentAddress = 'https://messenger.example.test/files/report';
let signings = 0;
let copies = 0;
const fetched: string[] = [];
const stored = new Map<string, Response>();
const keyOf = (address: string) => `${origin}/messenger-attachment-file/${encodeURIComponent(address)}`;
stored.set(keyOf(keptAddress), new Response(new Blob(['kept picture'], { type: 'image/png' })));

Reflect.set(globalThis, 'location', new URL(origin));
Reflect.set(globalThis, 'caches', {
	open: async () => ({
		match: async (key: string) => stored.get(key)?.clone(),
		put: async (key: string, response: Response) => { stored.set(key, response); },
		delete: async (key: string) => stored.delete(key)
	}),
	delete: async () => true
});
Reflect.set(globalThis, 'fetch', async (address: string) => {
	fetched.push(address);
	return new Response(new Blob(['new picture'], { type: 'image/png' }));
});
URL.createObjectURL = (file: Blob) => `blob:${origin}/${file.size}`;

mock.module('$lib/messenger/cache-scope', () => ({ onMessengerCacheReset: () => () => {} }));
mock.module('$lib/supabase', () => ({ isSupabaseConfigured: () => true, projectURL: () => 'https://project.example.test' }));
mock.module('$lib/messenger/kept-attachment', () => ({ keptAssetPathOf: () => null }));
mock.module('$lib/messenger/messenger-api', () => ({
	copyAttachmentForReading: async () => {
		copies += 1;
		return { address: `copy-${copies}` };
	}
}));
mock.module('$lib/transfer/company-transfer', () => ({
	readableForSeconds: 60 * 60,
	signedForReading: async () => {
		signings += 1;
		return `signed-${signings}`;
	}
}));

const { attachmentSource } = await import('../../../src/lib/stores/attachment-source.svelte');
const picture = (url: string) => ({ url, filename: 'picture.png', contentType: 'image/png', sizeBytes: 1, digest: 'digest' });

attachmentSource.wants([picture(keptAddress)]);
await nextTurn();
const keptOpenable = attachmentSource.openable(keptAddress);
const keptWork = { signings, copies };

attachmentSource.wants([picture(newAddress), { ...picture(documentAddress), contentType: 'application/pdf' }]);
await nextTurn();
await nextTurn();

const beforeNotShown = {
	keptOpenable,
	keptWork,
	newOpenable: attachmentSource.openable(newAddress),
	documentOpenable: attachmentSource.openable(documentAddress),
	fetched: [...fetched],
	newKept: stored.has(keyOf(newAddress)),
	documentKept: stored.has(keyOf(documentAddress))
};

attachmentSource.renew(keptAddress);
await nextTurn();
await nextTurn();

console.log(JSON.stringify({
	...beforeNotShown,
	notShownOpenable: attachmentSource.openable(keptAddress),
	notShownKeptAgain: await stored.get(keyOf(keptAddress))?.clone().text()
}));
