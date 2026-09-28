import { describe, expect, test } from 'bun:test';
import { attachmentStateOf } from '../../../src/lib/components/channel/channel-attachments';

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
