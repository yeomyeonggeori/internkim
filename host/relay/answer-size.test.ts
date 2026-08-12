import { describe, expect, test } from 'bun:test';
import {
	answerByteLength,
	defaultAnswerByteCeiling,
	largestMessageTheChannelCarries,
	largestRawBytesThatFit,
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

describe('the ceiling the channel actually has', () => {
	test('leaves the broadcast envelope room inside what Realtime carries', () => {
		expect(defaultAnswerByteCeiling).toBeLessThan(largestMessageTheChannelCarries);
	});

	test('is worth raising, because base64 costs a third of what it carries', () => {
		expect(largestRawBytesThatFit(defaultAnswerByteCeiling)).toBeGreaterThan(600_000);
	});
});
