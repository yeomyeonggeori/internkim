import { describe, expect, test } from 'bun:test';
import { costOfAnswer, creditsOfCost, streamCostReader } from '../../../../supabase/functions/_shared/model-call-cost.ts';

describe('what a model call costs in credits', () => {
	test('a credit is a thousandth of a dollar, and a part of one is a whole one', () => {
		expect(creditsOfCost(0.012)).toBe(12);
		expect(creditsOfCost(0.0123)).toBe(13);
		expect(creditsOfCost(0.0000001)).toBe(1);
		expect(creditsOfCost(0)).toBe(0);
	});

	test('the cost is read from an answer OpenRouter gave at once', () => {
		expect(costOfAnswer({ choices: [], usage: { total_tokens: 10, cost: 0.004 } })).toBe(0.004);
		expect(costOfAnswer({ choices: [] })).toBeNull();
	});

	test('the cost is read from the usage chunk at the end of a stream, however it was split', () => {
		const reader = streamCostReader();
		const stream =
			'data: {"choices":[{"delta":{"content":"안녕"}}]}\n\n' +
			': OPENROUTER PROCESSING\n\n' +
			'data: {"choices":[],"usage":{"total_tokens":42,"cost":0.0215}}\n\n' +
			'data: [DONE]\n\n';
		for (let start = 0; start < stream.length; start += 7) reader.read(stream.slice(start, start + 7));
		expect(reader.cost()).toBe(0.0215);
	});

	test('a stream that ends without a usage chunk has no cost', () => {
		const reader = streamCostReader();
		reader.read('data: {"choices":[{"delta":{"content":"hi"}}]}\n\ndata: [DONE]\n\n');
		expect(reader.cost()).toBeNull();
	});
});
