import { mock, setSystemTime } from 'bun:test';

Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
const nextTurn = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
const minute = 60 * 1000;
let clock = Date.parse('2026-10-12T09:00:00Z');
let signings = 0;
let isSigningRefused = false;

mock.module('$lib/messenger/cache-scope', () => ({ onMessengerCacheReset: () => () => {} }));
mock.module('$lib/supabase', () => ({ isSupabaseConfigured: () => true, projectURL: () => 'https://project.example.com' }));
mock.module('$lib/messenger/kept-attachment', () => ({ keptAssetPathOf: () => 'company/shared/transfer/digest' }));
mock.module('$lib/messenger/messenger-api', () => ({ copyAttachmentForReading: async () => ({ address: 'unused' }) }));
mock.module('$lib/messenger/attachment-files', () => ({
	isKeptAsFile: () => false,
	keptAttachmentFile: async () => '',
	keepAttachmentFile: async () => {},
	forgetAttachmentFile: async () => {},
	forgetAttachmentFiles: async () => {}
}));
mock.module('$lib/transfer/company-transfer', () => ({
	readableForSeconds: 60 * 60,
	signedForReading: async () => {
		await nextTurn();
		if (isSigningRefused) throw new Error('the store would not sign for this copy');
		signings += 1;
		return `signed-${signings}`;
	}
}));

const { attachmentSource } = await import('../../../src/lib/stores/attachment-source.svelte');
const attachment = { url: 'notes-url', filename: 'notes.pdf', contentType: 'application/pdf', sizeBytes: 1, digest: 'digest' };
const advance = (minutes: number) => {
	clock += minutes * minute;
	setSystemTime(new Date(clock));
};
const failureOf = (reading: Promise<string>) => reading.then(() => '', (failure: Error) => failure.message);

advance(0);
const unknown = await attachmentSource.readable('sent-blob-url', 'blob:sent');
attachmentSource.wants([attachment]);
const whileFresh = await attachmentSource.readable(attachment.url, 'held');
const signingsWhileFresh = signings;
advance(56);
const [afterAnHour, together] = await Promise.all([
	attachmentSource.readable(attachment.url, 'held'),
	attachmentSource.readable(attachment.url, 'held')
]);
const signingsAfterAnHour = signings;
advance(56);
isSigningRefused = true;
const refused = await failureOf(attachmentSource.readable(attachment.url, 'held'));

console.log(JSON.stringify({ unknown, whileFresh, signingsWhileFresh, afterAnHour, together, signingsAfterAnHour, refused }));
