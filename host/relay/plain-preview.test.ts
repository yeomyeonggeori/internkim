import { describe, expect, test } from 'bun:test';
import { plainPreviewOf } from './plain-preview';

describe('plainPreviewOf', () => {
	test('emphasis, strikethrough and inline code keep only their words', () => {
		expect(plainPreviewOf('**굵게** *기울임* ~~취소~~ `code`')).toBe('굵게 기울임 취소 code');
	});

	test('headings, paragraphs and quotes are one line each', () => {
		expect(plainPreviewOf('# 제목\n\n본문 문단\n\n> 인용한 말')).toBe('제목\n본문 문단\n인용한 말');
	});

	test('an unordered list is one bulleted line per item', () => {
		expect(plainPreviewOf('- 첫째\n- 둘째')).toBe('• 첫째\n• 둘째');
	});

	test('an ordered list keeps its real numbers', () => {
		expect(plainPreviewOf('3. 셋째\n4. 넷째')).toBe('3. 셋째\n4. 넷째');
	});

	test('a task list item reads like any other item', () => {
		expect(plainPreviewOf('- [ ] 할 일\n- [x] 한 일')).toBe('• 할 일\n• 한 일');
	});

	test('nested items are flattened in order', () => {
		expect(plainPreviewOf('- 바깥\n  - 안쪽\n- 다음')).toBe('• 바깥\n• 안쪽\n• 다음');
	});

	test('a link reads as its text and a bare address as written', () => {
		expect(plainPreviewOf('[문서](https://example.com/doc) 와 https://example.com/raw 와 <https://example.com/angle>')).toBe(
			'문서 와 https://example.com/raw 와 https://example.com/angle'
		);
	});

	test('an image reads as its alt text, or as nothing without one', () => {
		expect(plainPreviewOf('사진 ![회의실](https://example.com/a.png)')).toBe('사진 회의실');
		expect(plainPreviewOf('![](https://example.com/a.png)')).toBe('');
	});

	test('a fenced code block keeps its code without fences or language', () => {
		expect(plainPreviewOf('```js\nconst a = 1;\n```')).toBe('const a = 1;');
	});

	test('an indented code block keeps its code', () => {
		expect(plainPreviewOf('설명\n\n    run --now')).toBe('설명\nrun --now');
	});

	test('a table is one line per row, header first, without the separator row', () => {
		expect(plainPreviewOf('| 이름 | 시간 |\n|---|---|\n| 이샘플 | 3시 |\n| 박예시 | 4시 |')).toBe(
			'이름 · 시간\n이샘플 · 3시\n박예시 · 4시'
		);
	});

	test('a horizontal rule disappears', () => {
		expect(plainPreviewOf('위\n\n---\n\n아래')).toBe('위\n아래');
	});

	test('raw HTML is kept as written', () => {
		expect(plainPreviewOf('<div>상자</div>')).toBe('<div>상자</div>');
		expect(plainPreviewOf('a <b>b</b>')).toBe('a <b>b</b>');
	});

	test('blocks are separated by a single line, never a blank one', () => {
		expect(plainPreviewOf('\n\n첫 문단\n\n\n\n둘째 문단\n\n')).toBe('첫 문단\n둘째 문단');
	});

	test('ampersands, angle brackets and quotes come out literally, never as entities', () => {
		expect(plainPreviewOf('A & B "C" 1 < 2 > 0 `x && y`')).toBe('A & B "C" 1 < 2 > 0 x && y');
		expect(plainPreviewOf('| a & b | "c" |\n|---|---|\n| 1 < 2 | 3 > 2 |')).toBe('a & b · "c"\n1 < 2 · 3 > 2');
	});

	test('a message with no markdown comes out unchanged', () => {
		const message = '@이샘플 오늘 회의 30분 미뤄도 될까요? 🙏\n내일은 괜찮아요';

		expect(plainPreviewOf(message)).toBe(message);
	});
});
