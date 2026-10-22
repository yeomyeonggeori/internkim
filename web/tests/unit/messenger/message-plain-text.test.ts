import { describe, expect, test } from 'bun:test';
import { messagePlainText } from '$lib/messenger/message-plain-text';

describe('messagePlainText', () => {
	test('a briefing reads as its words without the marks that format it', () => {
		const briefing = '동하 님, 오늘 아침 브리핑입니다.\n\n**오늘 일정 (10/16 금)**\n- 미국 출장 (10/5~10/17)\n- `배포` [회의록](https://example.com/notes)';
		expect(messagePlainText(briefing)).toBe('동하 님, 오늘 아침 브리핑입니다. 오늘 일정 (10/16 금) 미국 출장 (10/5~10/17) 배포 회의록');
	});

	test('a heading, a quote and a code block keep their text', () => {
		expect(messagePlainText('## 결과\n\n> 통과했어요\n\n```\nok\n```')).toBe('결과 통과했어요 ok');
	});

	test('escaped marks stay as written and inline HTML tags are left out', () => {
		expect(messagePlainText('R&D <b>굵게</b> `a&b` \\*별\\* 5 > 3')).toBe('R&D 굵게 a&b *별* 5 > 3');
	});

	test('plain text passes through unchanged', () => {
		expect(messagePlainText('로그인 화면 고쳤어요')).toBe('로그인 화면 고쳤어요');
	});
});
