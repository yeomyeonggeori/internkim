import { mock, setSystemTime } from 'bun:test';

Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
const nextTurn = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
const minute = 60 * 1000;
let clock = Date.parse('2026-10-12T09:00:00Z');
let signings = 0;

mock.module('$lib/messenger/cache-scope', () => ({ onMessengerCacheReset: () => () => {} }));
mock.module('$lib/supabase', () => ({ isSupabaseConfigured: () => true, projectURL: () => 'https://project.example.com' }));
mock.module('$lib/messenger/kept-attachment', () => ({ keptAssetPathOf: () => 'company/shared/transfer/digest' }));
mock.module('$lib/messenger/messenger-api', () => ({ copyAttachmentForReading: async () => ({ address: 'unused' }) }));
mock.module('$lib/transfer/company-transfer', () => ({
	readableForSeconds: 60 * 60,
	signedForReading: async () => {
		signings += 1;
		return `signed-${signings}`;
	}
}));

const { attachmentSource } = await import('../../../src/lib/stores/attachment-source.svelte');
const attachment = { url: 'picture-url', filename: 'picture.png', contentType: 'image/png', sizeBytes: 1, digest: 'digest' };
const advance = async (minutes: number) => {
	clock += minutes * minute;
	setSystemTime(new Date(clock));
	await nextTurn();
};

await advance(0);
attachmentSource.wants([attachment]);
await nextTurn();
const first = attachmentSource.openable(attachment.url);
attachmentSource.renew(attachment.url);
attachmentSource.wants([attachment]);
await nextTurn();
const whileFresh = attachmentSource.openable(attachment.url);
await advance(56);
attachmentSource.renew(attachment.url);
await nextTurn();
const afterBrokenImage = attachmentSource.openable(attachment.url);
await advance(56);
attachmentSource.wants([attachment]);
await nextTurn();
const afterReopen = attachmentSource.openable(attachment.url);

console.log(JSON.stringify({ first, whileFresh, afterBrokenImage, afterReopen, signings }));
