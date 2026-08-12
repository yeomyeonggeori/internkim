import { describe, expect, test } from 'bun:test';
import {
	answerByteLength,
	defaultAnswerByteCeiling,
	largestMessageTheProPlanCarries,
	oversizeNotice
} from './answer-size';

const ceiling = 200_000;

describe('oversizeNotice', () => {
	test('an ordinary answer passes through untouched', () => {
		const answer = { callID: 'a', status: 200, body: { channels: ['general'] } };
		expect(oversizeNotice(answer, ceiling)).toBeNull();
	});

	test('an answer over the ceiling becomes a 413 naming the size', () => {
		const answer = { callID: 'a', status: 200, body: { dataURL: 'x'.repeat(ceiling) } };
		const notice = oversizeNotice(answer, ceiling);
		expect(notice?.status).toBe(413);
		expect(notice?.callID).toBe('a');
		expect(answerByteLength(notice!)).toBeLessThan(ceiling);
	});

	test('multi-byte characters are counted as bytes, not characters', () => {
		const answer = { callID: 'a', status: 200, body: { text: '가'.repeat(100) } };
		expect(answerByteLength(answer)).toBeGreaterThan(300);
	});
});

describe('the ceiling the plan actually has', () => {
	test('spends the whole of it, because httpSend wraps the answer in no envelope', () => {
		expect(defaultAnswerByteCeiling).toBe(largestMessageTheProPlanCarries);
	});

	test('an answer at the default ceiling is sent, and one byte past it is refused', () => {
		const room = defaultAnswerByteCeiling - answerByteLength({ callID: 'a', status: 200, body: { dataURL: '' } });
		const fits = { callID: 'a', status: 200, body: { dataURL: 'x'.repeat(room) } };

		expect(answerByteLength(fits)).toBe(defaultAnswerByteCeiling);
		expect(oversizeNotice(fits, defaultAnswerByteCeiling)).toBeNull();

		const over = { callID: 'a', status: 200, body: { dataURL: 'x'.repeat(room + 1) } };
		expect(oversizeNotice(over, defaultAnswerByteCeiling)?.status).toBe(413);
	});
});
