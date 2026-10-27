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
		return `signed-${signings}`;
	}
}));

const { attachmentSource } = await import('../../../src/lib/stores/attachment-source.svelte');
const attachment = { url: 'picture-url', filename: 'picture.png', contentType: 'image/png', sizeBytes: 1, digest: 'digest' };
const seen = () => ({
	status: attachmentSource.status(attachment.url),
	openable: attachmentSource.openable(attachment.url),
	notShown: attachmentSource.isNotShown(attachment.url)
});

attachmentSource.wants([attachment]);
await nextTurn();
const first = seen();
attachmentSource.renew(attachment.url);
const whileOpeningAgain = seen();
await nextTurn();
const afterFirstFailure = seen();
attachmentSource.renew(attachment.url);
const afterSecondFailure = seen();
attachmentSource.retry(attachment.url);
await nextTurn();
const afterTap = seen();

console.log(JSON.stringify({ first, whileOpeningAgain, afterFirstFailure, afterSecondFailure, afterTap, signings }));
