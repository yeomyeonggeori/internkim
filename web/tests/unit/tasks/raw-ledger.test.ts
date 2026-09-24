import { describe, expect, test } from 'bun:test';
import { filterLedger, groupLedger } from '../../../src/routes/runs/raw-ledger';
import type { TaskEvent } from '../../../src/routes/runs/runs-api';

function event(name: string, body: unknown = {}): TaskEvent {
	return { name, body: typeof body === 'string' ? body : JSON.stringify(body) };
}

const ledger = [
	event('task.created', '귤 소개 사이트 만들어줘'),
	event('llm.call', { kind: 'decision', latencyMs: 800 }),
	event('task.turn_input'),
	event('agent.launch_step.result'),
	event('llm.call', { kind: 'structured', latencyMs: 3000 }),
	event('agent.action', { action: 'continue', toolName: 'site_serve' }),
	event('tool.site_serve.requested'),
	event('tool.site_serve.result'),
	event('llm.call', { kind: 'structured', latencyMs: 1000 }),
	event('agent.action', { action: 'finish', message: '게시했습니다.' }),
	event('task.completed')
];

describe('the raw ledger', () => {
	test('groups events into intake, then each turn, then each step the model took', () => {
		const sections = groupLedger(ledger);

		expect(sections.map((section) => section.turnNumber)).toEqual([0, 1]);
		expect(sections[0].steps[0].entries.map((entry) => entry.event.name)).toEqual(['task.created', 'llm.call']);
		expect(sections[1].steps.map((step) => [step.number, step.toolName, step.action])).toEqual([
			[1, 'site_serve', 'continue'],
			[2, undefined, 'finish']
		]);
		expect(sections[1].steps[0].entries.map((entry) => entry.event.name)).toEqual([
			'task.turn_input',
			'agent.launch_step.result',
			'llm.call',
			'agent.action',
			'tool.site_serve.requested',
			'tool.site_serve.result'
		]);
		expect(sections[1].steps[1].entries.at(-1)?.event.name).toBe('task.completed');
	});

	test('gives each action its own step even when no model call separates them', () => {
		const sections = groupLedger([
			event('task.turn_input'),
			event('agent.action', { action: 'continue', toolName: 'site_serve' }),
			event('tool.site_serve.result'),
			event('agent.action', { action: 'continue' })
		]);

		expect(sections[1].steps.map((step) => [step.number, step.toolName, step.action])).toEqual([
			[1, 'site_serve', 'continue'],
			[2, undefined, 'continue']
		]);
	});

	test('keeps the structure while hiding what the filter leaves out', () => {
		const sections = filterLedger(groupLedger(ledger), (taskEvent) => taskEvent.name.startsWith('tool.'));

		expect(sections).toHaveLength(1);
		expect(sections[0].steps.map((step) => step.number)).toEqual([1]);
		expect(sections[0].steps[0].entries).toHaveLength(2);
	});
});
