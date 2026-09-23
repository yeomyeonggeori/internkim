import { parseJSON, readLLMCallRecord, readRecord, readString, type LLMCallRecord } from './llm-calls';
import type { TaskEvent } from './runs-api';

export type ModelCall = { event: TaskEvent; record: LLMCallRecord };

export type TaskStepKind = 'tool' | 'reply' | 'failure';

export type TaskStep = {
	key: string;
	kind: TaskStepKind;
	title: string;
	toolName?: string;
	isFailed: boolean;
	isFinished: boolean;
	failureText?: string;
	input?: unknown;
	output?: unknown;
	modelCalls: ModelCall[];
	turnInput?: TaskEvent;
	startedAt?: string;
	endedAt?: string;
};

export type TaskStory = {
	intakeDecision?: ModelCall;
	steps: TaskStep[];
};

type StoryDraft = {
	story: TaskStory;
	pendingCalls: ModelCall[];
	pendingTurnInput?: TaskEvent;
	pendingReason: string;
};

const toolEventPattern = /^tool\.(.+)\.(requested|result)$/;
const inputSummaryFields = ['title', 'name', 'subject', 'path', 'command', 'query', 'url', 'slug'];

export function buildTaskStory(taskEvents: TaskEvent[]): TaskStory {
	const draft: StoryDraft = { story: { steps: [] }, pendingCalls: [], pendingReason: '' };
	taskEvents.forEach((taskEvent, index) => absorbEvent(draft, taskEvent, index));
	closeWithUnansweredCalls(draft);
	return draft.story;
}

function absorbEvent(draft: StoryDraft, taskEvent: TaskEvent, index: number) {
	if (taskEvent.name === 'llm.call') return absorbModelCall(draft, taskEvent);
	if (taskEvent.name === 'task.turn_input') {
		draft.pendingTurnInput = taskEvent;
		return;
	}
	if (taskEvent.name === 'agent.action') return absorbAction(draft, taskEvent, index);
	if (taskEvent.name === 'task.failed' || taskEvent.name.startsWith('agent.failure')) return absorbFailure(draft, taskEvent, index);
	const toolEvent = toolEventPattern.exec(taskEvent.name);
	if (!toolEvent) return;
	if (toolEvent[2] === 'requested') return absorbToolRequest(draft, taskEvent, toolEvent[1], index);
	absorbToolResult(draft, taskEvent, toolEvent[1]);
}

function absorbModelCall(draft: StoryDraft, taskEvent: TaskEvent) {
	const record = readLLMCallRecord(taskEvent.body);
	if (!record) return;
	if (record.kind === 'decision' && !draft.story.intakeDecision) {
		draft.story.intakeDecision = { event: taskEvent, record };
		return;
	}
	draft.pendingCalls.push({ event: taskEvent, record });
}

function absorbAction(draft: StoryDraft, taskEvent: TaskEvent, index: number) {
	const action = readRecord(parseJSON(taskEvent.body));
	if (!action) return;
	draft.pendingReason = readString(action.reason) ?? readString(action.assistantText) ?? '';
	if (action.action !== 'finish' && action.final !== true) return;
	const message = readString(action.message) ?? '';
	pushStep(draft, taskEvent, index, { kind: 'reply', title: message, isFailed: false, isFinished: true, endedAt: taskEvent.createdAt });
}

function absorbToolRequest(draft: StoryDraft, taskEvent: TaskEvent, toolName: string, index: number) {
	const input = readRecord(parseJSON(taskEvent.body))?.input;
	const title = draft.pendingReason || inputSummary(input) || toolName;
	draft.pendingReason = '';
	pushStep(draft, taskEvent, index, { kind: 'tool', title, toolName, input, isFailed: false, isFinished: false });
}

function absorbToolResult(draft: StoryDraft, taskEvent: TaskEvent, toolName: string) {
	const step = draft.story.steps.findLast((candidate) => candidate.toolName === toolName && !candidate.isFinished);
	if (!step) return;
	const result = readRecord(parseJSON(taskEvent.body)) ?? {};
	const output = readRecord(result.output);
	const failure = readRecord(result.failure);
	step.isFinished = true;
	step.endedAt = taskEvent.createdAt;
	step.output = output?.data ?? output?.content ?? output;
	if (!failure) return;
	step.isFailed = true;
	step.failureText = readString(failure.message) ?? readString(output?.content) ?? readString(failure.kind);
	if (step.output === step.failureText) step.output = undefined;
}

function absorbFailure(draft: StoryDraft, taskEvent: TaskEvent, index: number) {
	const body = readRecord(parseJSON(taskEvent.body));
	const failureText = readString(body?.reason) ?? readString(body?.error) ?? (body ? undefined : taskEvent.body.trim());
	pushStep(draft, taskEvent, index, { kind: 'failure', title: failureText || taskEvent.name, isFailed: true, isFinished: true, failureText, endedAt: taskEvent.createdAt });
}

function closeWithUnansweredCalls(draft: StoryDraft) {
	const failedCall = draft.pendingCalls.find((call) => call.record.isError);
	if (!failedCall) return;
	const failureText = failedCall.record.error;
	pushStep(draft, failedCall.event, draft.story.steps.length, { kind: 'failure', title: failureText || failedCall.record.kind, isFailed: true, isFinished: true, failureText, endedAt: failedCall.event.createdAt });
}

function pushStep(draft: StoryDraft, taskEvent: TaskEvent, index: number, step: Omit<TaskStep, 'key' | 'modelCalls' | 'turnInput' | 'startedAt'>) {
	const modelCalls = draft.pendingCalls;
	draft.story.steps.push({
		...step,
		key: taskEvent.id ?? `${taskEvent.name}-${index}`,
		modelCalls,
		turnInput: draft.pendingTurnInput,
		startedAt: stepStart(modelCalls, taskEvent)
	});
	draft.pendingCalls = [];
	draft.pendingTurnInput = undefined;
}

function stepStart(modelCalls: ModelCall[], taskEvent: TaskEvent): string | undefined {
	const firstCall = modelCalls[0];
	if (!firstCall?.event.createdAt) return taskEvent.createdAt;
	const calledAt = Date.parse(firstCall.event.createdAt) - firstCall.record.latencyMs;
	return Number.isFinite(calledAt) ? new Date(calledAt).toISOString() : taskEvent.createdAt;
}

export function stepDurationMS(step: TaskStep): number | undefined {
	const duration = Date.parse(step.endedAt ?? '') - Date.parse(step.startedAt ?? '');
	return Number.isFinite(duration) && duration >= 0 ? duration : undefined;
}

function inputSummary(input: unknown): string {
	const fields = readRecord(input);
	if (!fields) return '';
	for (const field of inputSummaryFields) {
		const value = readString(fields[field]);
		if (value) return value;
	}
	return '';
}
