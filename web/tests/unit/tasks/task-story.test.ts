import { describe, expect, test } from 'bun:test';
import type { TaskEvent } from '../../../src/routes/runs/runs-api';
import { buildTaskStory, stepDurationMS } from '../../../src/routes/runs/task-story';

function event(name: string, body: unknown, createdAt: string, id?: string): TaskEvent {
	return { id, name, body: typeof body === 'string' ? body : JSON.stringify(body), createdAt };
}

const decisionCall = event('llm.call', { kind: 'decision', latencyMs: 800 }, '2026-06-17T01:00:01.000Z', 'decision-1');
const turnCall = event('llm.call', { kind: 'structured', latencyMs: 3000 }, '2026-06-17T01:00:05.000Z', 'turn-call-1');
const turnInput = event('task.turn_input', { $part: 'x' }, '2026-06-17T01:00:01.500Z', 'turn-input-1');

describe('a task story', () => {
	test('tells a tool call as one step: the model call that chose it, its reason, its input and its result', () => {
		const story = buildTaskStory([
			event('task.created', '귤 소개 이미지 만들어줘', '2026-06-17T01:00:00.000Z'),
			decisionCall,
			turnInput,
			turnCall,
			event('agent.action', { action: 'continue', toolName: 'image_generate', reason: '이미지를 만든다' }, '2026-06-17T01:00:05.100Z'),
			event('tool.image_generate.requested', { input: { prompt: '맛있는 귤 소개 이미지' } }, '2026-06-17T01:00:05.200Z', 'request-1'),
			event('tool.image_generate.result', { output: { data: { path: '/workspace/shared/tangerine.png' } } }, '2026-06-17T01:00:09.000Z'),
			event('agent.action', { action: 'finish', message: '만들었습니다.' }, '2026-06-17T01:00:10.000Z', 'finish-1')
		]);

		expect(story.intakeDecision?.event.id).toBe('decision-1');
		expect(story.steps.map((step) => [step.kind, step.title])).toEqual([
			['tool', '이미지를 만든다'],
			['reply', '만들었습니다.']
		]);
		const [toolStep] = story.steps;
		expect(toolStep.modelCalls.map((call) => call.event.id)).toEqual(['turn-call-1']);
		expect(toolStep.turnInput?.id).toBe('turn-input-1');
		expect(toolStep.output).toEqual({ path: '/workspace/shared/tangerine.png' });
		expect(stepDurationMS(toolStep)).toBe(7000);
	});

	test('names a tool call the model gave no reason for by what it was asked to act on', () => {
		const story = buildTaskStory([
			event('agent.action', { action: 'continue', toolName: 'file_read' }, '2026-06-17T01:00:00.000Z'),
			event('tool.file_read.requested', { input: { path: '회의록.md' } }, '2026-06-17T01:00:00.100Z')
		]);

		expect(story.steps[0]).toMatchObject({ title: '회의록.md', toolName: 'file_read', isFinished: false });
	});

	test('marks the step whose tool failed, with what failed', () => {
		const story = buildTaskStory([
			event('tool.shell.requested', { input: { command: 'ls /private' } }, '2026-06-17T01:00:00.000Z'),
			event('tool.shell.result', { output: { content: 'permission denied' }, failure: { kind: 'policy_blocked' } }, '2026-06-17T01:00:01.000Z')
		]);

		expect(story.steps[0]).toMatchObject({ isFailed: true, failureText: 'permission denied', output: undefined });
	});

	test('ends with the model call that failed when no step came of it', () => {
		const story = buildTaskStory([event('llm.call', { kind: 'structured', latencyMs: 0, isError: true, error: 'provider timed out' }, '2026-06-17T01:00:00.000Z')]);

		expect(story.steps).toHaveLength(1);
		expect(story.steps[0]).toMatchObject({ kind: 'failure', title: 'provider timed out' });
	});
});
