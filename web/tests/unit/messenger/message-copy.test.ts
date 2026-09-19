import { describe, expect, test } from 'bun:test';
import { openableAttachments, pictureAddressesOf } from '../../../src/lib/components/channel/channel-attachments';
import { whatToCopy } from '../../../src/lib/components/channel/message-copy';
import type { ChannelMessageAttachment } from '../../../src/lib/components/channel/channel-api';

const picture = (url: string, source?: string): ChannelMessageAttachment => ({ kind: 'image', url, source });
const file = (url: string, source?: string): ChannelMessageAttachment => ({ kind: 'file', url, source });

describe('what a message puts on the clipboard', () => {
	test('the words win when the message has any', () => {
		expect(whatToCopy('회의는 세 시입니다', ['https://example.com/one.png'])).toEqual({
			kind: 'text',
			text: '회의는 세 시입니다'
		});
	});

	test('a message of pictures alone gives its first picture', () => {
		expect(whatToCopy('', ['https://example.com/one.png', 'https://example.com/two.png'])).toEqual({
			kind: 'picture',
			address: 'https://example.com/one.png'
		});
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
		const addresses = pictureAddressesOf([
			picture('kept-one', 'https://example.com/one.png'),
			file('kept-report', 'https://example.com/report.pdf'),
			picture('kept-unsigned', ''),
			picture('kept-two', 'https://example.com/two.png')
		]);
		expect(addresses).toEqual(['https://example.com/one.png', 'https://example.com/two.png']);
	});
});
