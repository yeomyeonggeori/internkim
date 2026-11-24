import { describe, expect, test } from 'bun:test';
import { messagePicturesOf, openableAttachments } from '../../../src/lib/components/channel/channel-attachments';
import { whatToCopy } from '../../../src/lib/components/channel/message-copy';
import type { ChannelMessageAttachment } from '../../../src/lib/components/channel/channel-api';

const picture = (url: string, source?: string): ChannelMessageAttachment => ({ kind: 'image', url, source });
const file = (url: string, source?: string): ChannelMessageAttachment => ({ kind: 'file', url, source });
const shown = (name: string) => ({ address: `https://example.com/${name}.png`, filename: `${name}.png`, identity: `kept-${name}` });

describe('what a message puts on the clipboard', () => {
	test('the words win when the message has any', () => {
		expect(whatToCopy('회의는 세 시입니다', [shown('one')])).toEqual({
			kind: 'text',
			text: '회의는 세 시입니다'
		});
	});

	test('a message of pictures alone gives its first picture', () => {
		expect(whatToCopy('', [shown('one'), shown('two')])).toEqual({ kind: 'picture', picture: shown('one') });
	});

	test('a message with neither words nor pictures gives nothing', () => {
		expect(whatToCopy('', [])).toEqual({ kind: 'nothing' });
	});
});

describe('where the pictures of a message open from', () => {
	test('an attachment keeps its own source and the rest are looked up', () => {
		const opened = openableAttachments(
			[picture('kept-one', 'https://example.com/own.png'), picture('kept-two')],
			(url) => `https://example.com/signed/${url}`
		);
		expect(opened.map((attachment) => attachment.source)).toEqual([
			'https://example.com/own.png',
			'https://example.com/signed/kept-two'
		]);
	});

	test('only pictures that can be opened are listed, in the order they were sent', () => {
		const pictures = messagePicturesOf([
			picture('kept-one', 'https://example.com/one.png'),
			file('kept-report', 'https://example.com/report.pdf'),
			picture('kept-unsigned', ''),
			picture('kept-two', 'https://example.com/two.png')
		]);
		expect(pictures.map((shownPicture) => shownPicture.address)).toEqual(['https://example.com/one.png', 'https://example.com/two.png']);
	});
});
