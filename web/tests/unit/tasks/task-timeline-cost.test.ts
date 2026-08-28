import { describe, expect, test } from 'bun:test';
import { formatCostUSD, summarizeTimeline, taskDetailShareText, taskEventShareText, type TaskDetail, type TaskEvent } from '../../../src/routes/runs/runs-api';

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
		expect(Number(summary.llmCostUSD.toFixed(6))).toBe(0.0164);
		expect(summary.toolCallCount).toBe(1);
	});

	test('ignores events without cost fields', () => {
		const summary = summarizeTimeline([llmCallEvent({ totalTokens: 10 })]);
		expect(summary.llmCostUSD).toBe(0);
		expect(summary.llmCachedPromptTokens).toBe(0);
	});

	test('falls back to BYOK upstream cost when top-level cost is absent', () => {
		const summary = summarizeTimeline([
			llmCallEvent({ totalTokens: 17024, upstreamInferenceCostUSD: 0.0044195 }),
			llmCallEvent({ totalTokens: 350, upstreamInferenceCostUSD: 0.0002885 })
		]);
		expect(Number(summary.llmCostUSD.toFixed(7))).toBe(0.004708);
	});

	test('prefers top-level cost over upstream when both present', () => {
		const summary = summarizeTimeline([llmCallEvent({ costUSD: 0.02, upstreamInferenceCostUSD: 0.05 })]);
		expect(summary.llmCostUSD).toBe(0.02);
	});
});

describe('formatCostUSD', () => {
	test('uses four decimals for sub-cent costs and two for larger', () => {
		expect(formatCostUSD(0)).toBe('$0');
		expect(formatCostUSD(0.0155)).toBe('$0.0155');
		expect(formatCostUSD(1.239)).toBe('$1.24');
	});
});

describe('task detail share text', () => {
	test('formats task run and events for copying into another AI', () => {
		const detail: TaskDetail = {
			taskRun: {
				taskRunID: 'task-1',
				status: 'completed',
				prompt: '요청',
				result: 'completed',
				createdAt: '2026-06-25T01:00:00Z',
				updatedAt: '2026-06-25T01:01:00Z'
			},
			taskEvents: [
				{
					name: 'llm.call',
					body: JSON.stringify({ model: 'test-model', totalTokens: 12 }),
					createdAt: '2026-06-25T01:00:30Z'
				}
			]
		};

		const shareText = taskDetailShareText(detail);

		expect(shareText.includes('# Task Run task-1')).toBe(true);
		expect(shareText.includes('- prompt: 요청')).toBe(true);
		expect(shareText.includes('### Event 1: llm.call')).toBe(true);
		expect(shareText.includes('"totalTokens": 12')).toBe(true);
	});

	test('uses a longer markdown fence when event body contains backticks', () => {
		const shareText = taskEventShareText({ name: 'agent.action', body: '```nested```' });

		expect(shareText.includes('````text')).toBe(true);
		expect(shareText.trim().endsWith('````')).toBe(true);
	});
});
