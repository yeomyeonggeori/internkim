import { adminApiFetch } from '$lib/admin-api';
import { callCompanyApp } from '$lib/host-bridge';
import { isSupabaseConfigured } from '$lib/supabase';

export type DecisionAnswer = {
	type: string;
	choice?: string;
	noul?: number;
	probabilities?: Record<string, number>;
};

export type LLMCallRecord = {
	kind: string;
	schemaName?: string;
	model?: string;
	provider?: string;
	latencyMs: number;
	promptTokens: number;
	completionTokens: number;
	costUSD: number;
	isError: boolean;
	error?: string;
	usedFallback: boolean;
	fallbackReason?: string;
	decisionAnswers: Record<string, DecisionAnswer>;
	decisionDraws: Record<string, number>;
	decidedMessageIDs: string[];
};

export type DecisionRow = {
	question: string;
	answer: string;
	probability?: number;
	draw?: number;
};

export type ExchangeToolCall = { name: string; arguments: string };

export type ExchangeMessage = {
	role: string;
	text: string;
	reasoning?: string;
	imageCount: number;
	toolCalls: ExchangeToolCall[];
};

export type Exchange = {
	messages: ExchangeMessage[];
	toolNames: string[];
	schemaName?: string;
	seed?: number;
	servedBy?: string;
	decisionState?: unknown;
	decisionQuestions: string[];
	answer?: ExchangeMessage;
	request: string;
	response: string;
	input?: unknown;
};

export type InboundOutcome = 'task' | 'reacted' | 'ignored' | 'pending' | 'failed';

export type InboundMessage = {
	messageID: string;
	senderName: string;
	promptPreview: string;
	ingestedAt: string;
	outcome: InboundOutcome;
	ignoreReason?: string;
	taskRunID?: string;
	decision?: LLMCallRecord;
	reactionProbability?: number;
	reactionDraw?: number;
	reactionEmoji?: string;
};

export function readLLMCallRecord(body: string): LLMCallRecord | undefined {
	return readLLMCallRecordValue(parseJSON(body));
}

function readLLMCallRecordValue(value: unknown): LLMCallRecord | undefined {
	const record = readRecord(value);
	if (!record || typeof record.kind !== 'string') return undefined;
	return {
		kind: record.kind,
		schemaName: readString(record.schemaName),
		model: readString(record.model),
		provider: readString(record.upstreamProvider) ?? readString(record.provider),
		latencyMs: readNumber(record.latencyMs),
		promptTokens: readNumber(record.promptTokens),
		completionTokens: readNumber(record.completionTokens),
		costUSD: readNumber(record.costUSD),
		isError: record.isError === true,
		error: readString(record.error),
		usedFallback: record.usedFallback === true,
		fallbackReason: readString(record.fallbackReason),
		decisionAnswers: readDecisionAnswers(record.decisionAnswers),
		decisionDraws: readNumberMap(record.decisionDraws),
		decidedMessageIDs: Array.isArray(record.decidedMessageIDs) ? record.decidedMessageIDs.filter((messageID) => typeof messageID === 'string') : []
	};
}

export function decisionMessageKeys(record: LLMCallRecord): string[] {
	const keys = new Set(Object.keys(record.decisionAnswers).map((questionKey) => questionKey.split('.')[0]));
	return [...keys].sort();
}

export function messageKeyOf(record: LLMCallRecord, messageID: string): string | undefined {
	const index = record.decidedMessageIDs.indexOf(messageID);
	return index < 0 ? undefined : `m${index + 1}`;
}

export function decisionRows(record: LLMCallRecord, messageKey: string): DecisionRow[] {
	const prefix = `${messageKey}.`;
	return Object.entries(record.decisionAnswers)
		.filter(([questionKey]) => questionKey.startsWith(prefix))
		.map(([questionKey, answer]) => decisionRow(questionKey.slice(prefix.length), answer, record.decisionDraws[questionKey]))
		.sort((left, right) => left.question.localeCompare(right.question));
}

function decisionRow(question: string, answer: DecisionAnswer, draw: number | undefined): DecisionRow {
	if (answer.type === 'noul' && answer.noul !== undefined) {
		return { question, answer: answer.noul >= 0.5 ? 'yes' : 'no', probability: answer.noul, draw };
	}
	const choice = answer.choice ?? mostLikelyChoice(answer.probabilities);
	return { question, answer: choice ?? '—', probability: choice ? answer.probabilities?.[choice] : undefined, draw };
}

function mostLikelyChoice(probabilities: Record<string, number> | undefined): string | undefined {
	if (!probabilities) return undefined;
	return Object.entries(probabilities).sort((left, right) => right[1] - left[1])[0]?.[0];
}

export function reactionOf(record: LLMCallRecord, messageKey: string) {
	const reaction = record.decisionAnswers[`${messageKey}.reaction`];
	return {
		probability: reaction?.probabilities?.react,
		draw: record.decisionDraws[`${messageKey}.reaction`],
		emoji: record.decisionAnswers[`${messageKey}.reactionEmoji`]?.choice
	};
}

export function readExchange(document: unknown): Exchange {
	const stored = readRecord(document) ?? {};
	const requestBytes = typeof stored.request === 'string' ? stored.request : '';
	const responseBytes = typeof stored.response === 'string' ? stored.response : '';
	const request = readRecord(parseJSON(requestBytes)) ?? {};
	const response = readRecord(parseJSON(responseBytes));
	return {
		messages: readExchangeMessages(request),
		toolNames: readToolNames(request.tools),
		schemaName: readString(readRecord(readRecord(request.response_format)?.json_schema)?.name),
		seed: typeof request.seed === 'number' ? request.seed : undefined,
		servedBy: readString(response?.provider),
		decisionState: request.state,
		decisionQuestions: Object.keys(readRecord(request.questions) ?? {}).sort(),
		answer: response ? readExchangeAnswer(response) : undefined,
		request: requestBytes,
		response: responseBytes,
		input: typeof stored.input === 'string' ? parseJSON(stored.input) : undefined
	};
}

function readExchangeMessages(request: Record<string, unknown>): ExchangeMessage[] {
	if (typeof request.prompt === 'string') return [{ role: 'user', text: request.prompt, imageCount: 0, toolCalls: [] }];
	if (!Array.isArray(request.messages)) return [];
	return request.messages.flatMap((entry) => {
		const message = readRecord(entry);
		return message ? [readExchangeMessage(message)] : [];
	});
}

function readExchangeMessage(message: Record<string, unknown>): ExchangeMessage {
	const parts = Array.isArray(message.content) ? message.content.map(readRecord).filter((part) => part !== undefined) : [];
	const partText = parts.map((part) => readString(part.text) ?? '').filter(Boolean);
	return {
		role: readString(message.role) ?? 'user',
		text: [readString(message.content) ?? '', ...partText].filter(Boolean).join('\n\n'),
		reasoning: readString(message.reasoning),
		imageCount: parts.filter((part) => part.type === 'image_url').length,
		toolCalls: readToolCalls(message.tool_calls)
	};
}

function readExchangeAnswer(response: Record<string, unknown>): ExchangeMessage | undefined {
	const choice = Array.isArray(response.choices) ? readRecord(response.choices[0]) : undefined;
	const message = readRecord(choice?.message);
	return message ? readExchangeMessage(message) : undefined;
}

function readToolCalls(entries: unknown): ExchangeToolCall[] {
	if (!Array.isArray(entries)) return [];
	return entries.flatMap((entry) => {
		const call = readRecord(readRecord(entry)?.function);
		return call ? [{ name: readString(call.name) ?? '?', arguments: prettyArguments(readString(call.arguments) ?? '') }] : [];
	});
}

function readToolNames(entries: unknown): string[] {
	if (!Array.isArray(entries)) return [];
	return entries.flatMap((entry) => {
		const name = readString(readRecord(readRecord(entry)?.function)?.name);
		return name ? [name] : [];
	});
}

function prettyArguments(argumentsDocument: string): string {
	const parsed = parseJSON(argumentsDocument);
	return parsed === undefined ? argumentsDocument : JSON.stringify(parsed, undefined, 2);
}

export function readInboundMessages(document: unknown): InboundMessage[] {
	if (!Array.isArray(document)) return [];
	return document.flatMap((entry) => {
		const diagnostic = readRecord(entry);
		const messageID = readString(diagnostic?.externalMessageID);
		return diagnostic && messageID ? [readInboundMessage(diagnostic, messageID)] : [];
	});
}

function readInboundMessage(diagnostic: Record<string, unknown>, messageID: string): InboundMessage {
	const result = readRecord(diagnostic.result) ?? {};
	const decision = readLLMCallRecordValue(diagnostic.decision);
	const messageKey = decision ? messageKeyOf(decision, messageID) : undefined;
	const reaction = decision && messageKey ? reactionOf(decision, messageKey) : undefined;
	return {
		messageID,
		senderName: readString(diagnostic.senderName) ?? '',
		promptPreview: readString(diagnostic.promptPreview) ?? '',
		ingestedAt: readString(diagnostic.ingestedAt) ?? '',
		outcome: inboundOutcome(readString(diagnostic.connectorStatus), result),
		ignoreReason: readString(result.reason),
		taskRunID: readString(result.taskRunID),
		decision,
		reactionProbability: reaction?.probability,
		reactionDraw: reaction?.draw,
		reactionEmoji: reaction?.emoji
	};
}

function inboundOutcome(connectorStatus: string | undefined, result: Record<string, unknown>): InboundOutcome {
	if (readString(result.taskRunID)) return 'task';
	if (result.ignored === true) return result.reason === 'addressing_react_only' ? 'reacted' : 'ignored';
	if (connectorStatus === 'failed') return 'failed';
	return 'pending';
}

export async function fetchLLMCallExchange(llmCallID: string): Promise<Exchange> {
	const answer = isSupabaseConfigured() ? await askTheCompanyApp('person.runs.llm_call', { id: llmCallID }) : await askTheDevice('/runs/api/llm-call', { id: llmCallID });
	return readExchange(answer);
}

export async function fetchTurnInput(taskEventID: string): Promise<unknown> {
	return isSupabaseConfigured() ? askTheCompanyApp('person.runs.turn_input', { id: taskEventID }) : askTheDevice('/runs/api/turn-input', { id: taskEventID });
}

export async function fetchInboundMessages(limit: number): Promise<InboundMessage[]> {
	const answer = isSupabaseConfigured() ? await askTheCompanyApp('person.runs.inbound', { limit }) : await askTheDevice('/runs/api/inbound', { limit: String(limit) });
	return readInboundMessages(answer);
}

async function askTheDevice(path: string, parameters: Record<string, string>): Promise<unknown> {
	const response = await adminApiFetch(`${path}?${new URLSearchParams(parameters).toString()}`);
	if (!response.ok) throw new Error((await response.text()) || `${path} returned ${response.status}`);
	return response.json();
}

async function askTheCompanyApp(capability: string, body: Record<string, unknown>): Promise<unknown> {
	const answer = await callCompanyApp({ capability, body });
	if (answer.status >= 400) throw new Error(errorTextOf(answer.body) ?? `${capability} returned ${answer.status}`);
	return answer.body;
}

function errorTextOf(body: unknown): string | undefined {
	if (typeof body === 'string') return body;
	return readString(readRecord(body)?.error);
}

function readDecisionAnswers(value: unknown): Record<string, DecisionAnswer> {
	const answers: Record<string, DecisionAnswer> = {};
	for (const [questionKey, entry] of Object.entries(readRecord(value) ?? {})) {
		const answer = readRecord(entry);
		if (!answer || typeof answer.type !== 'string') continue;
		answers[questionKey] = {
			type: answer.type,
			choice: readString(answer.choice),
			noul: typeof answer.noul === 'number' ? answer.noul : undefined,
			probabilities: readNumberMap(answer.probabilities)
		};
	}
	return answers;
}

function readNumberMap(value: unknown): Record<string, number> {
	const numbers: Record<string, number> = {};
	for (const [key, entry] of Object.entries(readRecord(value) ?? {})) {
		if (typeof entry === 'number' && Number.isFinite(entry)) numbers[key] = entry;
	}
	return numbers;
}

function readNumber(value: unknown): number {
	return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

function readString(value: unknown): string | undefined {
	return typeof value === 'string' && value.trim() !== '' ? value : undefined;
}

function readRecord(value: unknown): Record<string, unknown> | undefined {
	if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
	return value as Record<string, unknown>;
}

function parseJSON(document: string): unknown {
	try {
		return JSON.parse(document);
	} catch {
		return undefined;
	}
}
