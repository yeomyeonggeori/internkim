import { parseJSON, readLLMCallRecord, readRecord, readString } from './llm-calls';
import type { TaskEvent } from './runs-api';

export type LedgerEntry = { event: TaskEvent; index: number };

export type LedgerStep = { number: number; toolName?: string; action?: string; entries: LedgerEntry[] };

export type LedgerSection = { turnNumber: number; steps: LedgerStep[] };

export function groupLedger(taskEvents: TaskEvent[]): LedgerSection[] {
	const sections: LedgerSection[] = [{ turnNumber: 0, steps: [{ number: 0, entries: [] }] }];
	taskEvents.forEach((event, index) => {
		if (event.name === 'task.turn_input') sections.push({ turnNumber: sections.length, steps: [] });
		const section = sections[sections.length - 1];
		const step = stepFor(section, event);
		step.entries.push({ event, index });
		if (event.name === 'agent.action') describeStep(step, event);
	});
	return sections;
}

function stepFor(section: LedgerSection, event: TaskEvent): LedgerStep {
	const current = section.steps.at(-1);
	if (section.turnNumber === 0 && current) return current;
	if (current && !(startsAStep(event) && current.action !== undefined)) return current;
	const step: LedgerStep = { number: section.steps.length + 1, entries: [] };
	section.steps.push(step);
	return step;
}

function startsAStep(event: TaskEvent): boolean {
	if (event.name === 'agent.action') return true;
	if (event.name !== 'llm.call') return false;
	return readLLMCallRecord(event.body)?.kind !== 'decision';
}

function describeStep(step: LedgerStep, event: TaskEvent) {
	const action = readRecord(parseJSON(event.body));
	step.action = readString(action?.action) ?? '';
	step.toolName = readString(action?.toolName);
}

export function isProminentEvent(event: TaskEvent): boolean {
	if (prominentEventNames.has(event.name)) return true;
	return event.name.startsWith('tool.') || event.name.startsWith('agent.failure') || event.name.startsWith('connector.reply');
}

const prominentEventNames = new Set(['task.created', 'task.turn_input', 'llm.call', 'agent.action', 'task.completed', 'task.failed', 'agent.recovery_attempt']);

export function eventTitle(event: TaskEvent): string {
	if (event.name !== 'agent.action') return event.name;
	const action = readRecord(parseJSON(event.body));
	const chosen = readString(action?.toolName) ?? readString(action?.action);
	return chosen ? `${event.name} · ${chosen}` : event.name;
}

export function filterLedger(sections: LedgerSection[], isShown: (event: TaskEvent) => boolean): LedgerSection[] {
	return sections
		.map((section) => ({
			...section,
			steps: section.steps.map((step) => ({ ...step, entries: step.entries.filter((entry) => isShown(entry.event)) })).filter((step) => step.entries.length > 0)
		}))
		.filter((section) => section.steps.length > 0);
}
