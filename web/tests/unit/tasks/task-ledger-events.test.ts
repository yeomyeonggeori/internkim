import { describe, expect, test } from 'bun:test';
import { eventLane, formatEventBody, readTaskDetail, summarizeTimeline } from '../../../src/routes/runs/runs-api';

const relayAnswer = {
	taskRun: {
		taskRunID: 'run-12',
		status: 'completed',
		requesterPersonID: 'person-3',
		requesterDisplayName: '박예시',
		prompt: '이번 주 회의록 정리해줘',
		result: '정리했습니다.',
		createdAt: '2026-09-01T01:00:00Z',
		updatedAt: '2026-09-01T01:04:00Z'
	},
	taskSteps: [{ taskStepID: 'step-1' }],
	taskEvents: [
		{ name: 'llm.call', body: '{"latencyMs":420,"totalTokens":900,"costUSD":0.0012}', createdAt: '2026-09-01T01:01:00Z' },
		{ name: 'tool.document_write.result', body: '{"path":"/workspace/shared/minutes.md"}' },
		{ name: 'agent.failure.tool_rejected', body: 'the tool refused' },
		{ name: 'connector.reply_sent', body: '{}' },
		{ body: 'an event with no name' },
		'not an event at all'
	]
};

describe('the ledger a run detail page renders, read out of a relay answer', () => {
	test('keeps every named event and drops what is not one', () => {
		const detail = readTaskDetail(relayAnswer);

		expect(detail.taskRun.taskRunID).toBe('run-12');
		expect(detail.taskEvents.map((taskEvent) => taskEvent.name)).toEqual([
			'llm.call',
			'tool.document_write.result',
			'agent.failure.tool_rejected',
			'connector.reply_sent'
		]);
	});

	test('sorts each event into the lane the timeline filters by', () => {
		const lanes = readTaskDetail(relayAnswer).taskEvents.map((taskEvent) => eventLane(taskEvent.name));

		expect(lanes).toEqual(['llm', 'tool', 'failure', 'other']);
	});

	test('counts the calls and the cost the summary strip shows', () => {
		const summary = summarizeTimeline(readTaskDetail(relayAnswer).taskEvents);

		expect(summary.llmCallCount).toBe(1);
		expect(summary.llmLatencyMS).toBe(420);
		expect(summary.llmTotalTokens).toBe(900);
		expect(summary.toolCallCount).toBe(1);
		expect(Number(summary.llmCostUSD.toFixed(4))).toBe(0.0012);
	});

	test('pretty-prints a JSON body and leaves a plain one alone', () => {
		const [llmCall, , failure] = readTaskDetail(relayAnswer).taskEvents;

		expect(formatEventBody(llmCall.body)).toContain('\n  "latencyMs": 420');
		expect(formatEventBody(failure.body)).toBe('the tool refused');
	});

	test('an event with no body still renders as an empty one', () => {
		const detail = readTaskDetail({ taskRun: relayAnswer.taskRun, taskEvents: [{ name: 'ask.requested' }] });

		expect(detail.taskEvents).toEqual([{ name: 'ask.requested', body: '', createdAt: undefined }]);
	});

	test('an answer with no run at all is refused rather than half-rendered', () => {
		expect(() => readTaskDetail({ taskEvents: [] })).toThrow('Task detail response was malformed');
	});
});
