import { mock } from 'bun:test';

Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
const nextTurn = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
let signings = 0;

mock.module('$lib/messenger/cache-scope', () => ({ onMessengerCacheReset: () => () => {} }));
mock.module('$lib/supabase', () => ({ isSupabaseConfigured: () => true, projectURL: () => 'https://project.example.com' }));
mock.module('$lib/messenger/kept-attachment', () => ({ keptAssetPathOf: () => 'company/shared/transfer/digest' }));
mock.module('$lib/messenger/messenger-api', () => ({ copyAttachmentForReading: async () => ({ address: 'unused' }) }));
mock.module('$lib/transfer/company-transfer', () => ({
	readableForSeconds: 60 * 60,
	signedForReading: async () => {
		signings += 1;
		if (signings === 1) throw new Error('Object not found');
		return `signed-${signings}`;
	}
}));

const { attachmentSource } = await import('../../../src/lib/stores/attachment-source.svelte');
const attachment = { url: 'picture-url', filename: 'picture.png', contentType: 'image/png', sizeBytes: 1, digest: 'digest' };

attachmentSource.wants([attachment]);
await nextTurn();
const afterFirstRead = { status: attachmentSource.status(attachment.url), failure: attachmentSource.failure(attachment.url) };
attachmentSource.wants([attachment]);
const whileReadingAgain = attachmentSource.status(attachment.url);
await nextTurn();
const afterNextRead = { status: attachmentSource.status(attachment.url), openable: attachmentSource.openable(attachment.url) };
attachmentSource.wants([attachment]);
await nextTurn();

console.log(JSON.stringify({ afterFirstRead, whileReadingAgain, afterNextRead, signings }));
