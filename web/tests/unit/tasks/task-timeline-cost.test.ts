import { describe, expect, test } from 'bun:test';
import { formatCostUSD, summarizeTimeline, type TaskEvent } from '../../../src/routes/tasks/tasks-api';

function llmCallEvent(body: Record<string, unknown>): TaskEvent {
	return { name: 'llm.call', body: JSON.stringify(body) };
}

describe('summarizeTimeline cost and cache accounting', () => {
	test('sums cost, cached tokens, and total tokens across llm.call events', () => {
		const events: TaskEvent[] = [
			llmCallEvent({ totalTokens: 12159, cachedPromptTokens: 128, costUSD: 0.0155 }),
			llmCallEvent({ totalTokens: 800, cachedPromptTokens: 600, costUSD: 0.0009 }),
			{ name: 'tool.terminal.result', body: '{}' }
		];

		const summary = summarizeTimeline(events);

		expect(summary.llmCallCount).toBe(2);
		expect(summary.llmTotalTokens).toBe(12959);
		expect(summary.llmCachedPromptTokens).toBe(728);
		expect(summary.llmCostUSD).toBeCloseTo(0.0164, 6);
		expect(summary.toolCallCount).toBe(1);
	});

	test('ignores events without cost fields', () => {
		const summary = summarizeTimeline([llmCallEvent({ totalTokens: 10 })]);
		expect(summary.llmCostUSD).toBe(0);
		expect(summary.llmCachedPromptTokens).toBe(0);
	});
});

describe('formatCostUSD', () => {
	test('uses four decimals for sub-cent costs and two for larger', () => {
		expect(formatCostUSD(0)).toBe('$0');
		expect(formatCostUSD(0.0155)).toBe('$0.0155');
		expect(formatCostUSD(1.239)).toBe('$1.24');
	});
});
