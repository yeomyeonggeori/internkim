import { describe, expect, test } from 'bun:test';
import { attachmentStateOf, formatAttachmentMeta, formatReceivedBytes } from '../../../src/lib/components/channel/channel-attachments';

describe('what an attachment card says while its file is fetched', () => {
	test('shimmers while the relay is still copying the file', () => {
		expect(attachmentStateOf(undefined, 'loading')).toBe('processing');
	});

	test('says it could not be loaded once nothing came back', () => {
		expect(attachmentStateOf(undefined, 'failed')).toBe('error');
	});

	test('is plain once there is an address to open', () => {
		expect(attachmentStateOf('https://company.supabase.co/signed', 'ready')).toBe('done');
	});

	test('a file the message itself addresses never shimmers', () => {
		expect(attachmentStateOf('https://relay.example.com/file.png', 'unasked')).toBe('done');
		expect(attachmentStateOf(undefined, 'unasked')).toBe('done');
	});
});

describe('what an attachment card says the file is', () => {
	test('names an office file by its extension, never its long media type', () => {
		const spreadsheetType = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet';
		expect(formatAttachmentMeta({ mimeType: spreadsheetType, filename: 'quote-items.xlsx', sizeBytes: 18_432 })).toBe('XLSX · 18 KB');
	});

	test('falls back to the media type when the file has no extension', () => {
		expect(formatAttachmentMeta({ mimeType: 'application/pdf', filename: 'quote' })).toBe('PDF');
	});
});

describe('what an attachment card says while it is downloaded', () => {
	test('counts what arrived in the unit of the whole file', () => {
		expect(formatReceivedBytes(10_171_187, 24_222_105)).toBe('9.7 / 23.1 MB');
		expect(formatReceivedBytes(512, 18_432)).toBe('1 / 18 KB');
	});

	test('counts only what arrived when the size is unknown', () => {
		expect(formatReceivedBytes(2_097_152, 0)).toBe('2.0 MB');
	});
});
