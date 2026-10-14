import { describe, expect, test } from 'bun:test';
import { markdownKeepingBlankLines } from '$lib/messenger/message-blank-lines';

const placeholder = ' ';

describe('a message keeps the blank lines it was written with', () => {
	test('a single line break and a paragraph break stay as written', () => {
		expect(markdownKeepingBlankLines('위\n아래')).toBe('위\n아래');
		expect(markdownKeepingBlankLines('첫 문단\n\n둘째 문단')).toBe('첫 문단\n\n둘째 문단');
	});

	test('each blank line past the paragraph break becomes a line of its own', () => {
		expect(markdownKeepingBlankLines('위\n\n\n\n아래')).toBe(
			`위\n\n${placeholder}\n\n${placeholder}\n\n아래`
		);
	});

	test('blank lines before the first line and after the last are dropped', () => {
		expect(markdownKeepingBlankLines('\n\n\n위\n\n\n\n')).toBe('위');
	});

	test('blank lines inside a code fence are left alone', () => {
		const fenced = '```\na\n\n\n\nb\n```';
		expect(markdownKeepingBlankLines(fenced)).toBe(fenced);
		expect(markdownKeepingBlankLines('~~~~\na\n```\n\n\n\nb\n~~~~')).toBe('~~~~\na\n```\n\n\n\nb\n~~~~');
	});
});
