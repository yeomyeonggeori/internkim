import { describe, expect, test } from 'bun:test';
import { bodyWithAttachmentLabels } from '../../../supabase/functions/_shared/attachment-labels.ts';

describe('a notification for a message carrying files', () => {
	test('names each kind of file in the reader language', () => {
		expect(bodyWithAttachmentLabels('', ['image'], 'ko')).toBe('[사진]');
		expect(bodyWithAttachmentLabels('', ['video'], 'en')).toBe('[Video]');
		expect(bodyWithAttachmentLabels('', ['file'], 'en')).toBe('[File]');
	});

	test('follows the words that came with the files, naming each kind once', () => {
		expect(bodyWithAttachmentLabels('회의 자료야', ['image', 'image', 'file'], 'ko')).toBe('회의 자료야 [사진] [파일]');
	});

	test('names a kind it does not know as a file', () => {
		expect(bodyWithAttachmentLabels('', ['audio'], 'ko')).toBe('[파일]');
	});
});
