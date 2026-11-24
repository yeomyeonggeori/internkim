import { mock, setSystemTime } from 'bun:test';

Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
const minute = 60 * 1000;
let clock = Date.parse('2026-10-12T09:00:00Z');
let copies = 0;
let signings = 0;
const goneCopies = new Set<string>();

mock.module('$lib/messenger/cache-scope', () => ({ onMessengerCacheReset: () => () => {} }));
mock.module('$lib/supabase', () => ({ isSupabaseConfigured: () => true, projectURL: () => 'https://project.example.com' }));
mock.module('$lib/messenger/kept-attachment', () => ({ keptAssetPathOf: () => null }));
mock.module('$lib/messenger/messenger-api', () => ({
	copyAttachmentForReading: async () => {
		copies += 1;
		return { address: `copy-${copies}` };
	}
}));
mock.module('$lib/messenger/attachment-files', () => ({
	isKeptAsFile: () => false,
	keptAttachmentFile: async () => '',
	keepAttachmentFile: async () => {},
	forgetAttachmentFile: async () => {},
	forgetAttachmentFiles: async () => {}
}));
mock.module('$lib/transfer/company-transfer', () => ({
	readableForSeconds: 60 * 60,
	signedForReading: async (address: string) => {
		if (goneCopies.has(address)) throw new Error('Object not found');
		signings += 1;
		return `${address}-signed-${signings}`;
	}
}));
console.warn = () => {};

const { attachmentSource } = await import('../../../src/lib/stores/attachment-source.svelte');
const attachment = { url: 'messenger-file', filename: 'notes.pdf', contentType: 'application/pdf', sizeBytes: 1, digest: 'digest' };
const advance = (minutes: number) => {
	clock += minutes * minute;
	setSystemTime(new Date(clock));
};

advance(0);
attachmentSource.wants([attachment]);
const first = await attachmentSource.readable(attachment.url, '');
advance(56);
const afterAnHour = await attachmentSource.readable(attachment.url, '');
const copiesAfterAnHour = copies;
goneCopies.add('copy-1');
advance(56);
const afterTheCopyExpired = await attachmentSource.readable(attachment.url, '');

console.log(JSON.stringify({ first, afterAnHour, copiesAfterAnHour, afterTheCopyExpired, copies }));
