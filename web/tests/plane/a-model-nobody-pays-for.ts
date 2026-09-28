import { openSync } from 'node:fs';

export type AddressingDecision = {
	target: 'bot' | 'human' | 'anyone' | 'none' | 'unclear';
	shouldRespond: boolean;
};

export type TurnDecision = {
	route:
		| 'start_task'
		| 'continue_task'
		| 'revise_task'
		| 'answer_question'
		| 'answer_meta'
		| 'clarify'
		| 'consume'
		| 'give_up';
	classification?: 'quick_reply' | 'bounded_task' | 'needs_confirmation' | 'unsupported';
	taskShape?: string;
	level?: string;
	responseLanguage?: string;
	initialToolNames?: string[];
	expectedToolCount?: 'none' | 'one' | 'several';
	isExternalSendRequested?: boolean;
	approval?: 'approve' | 'approve_task' | 'reject' | 'unclear';
};

/** The message a turn decides, and the intaketest.Outcome the decisions endpoint answers for it. */
export type TurnOutcome = {
	message: string;
	addressing?: AddressingDecision;
	turnDecision?: TurnDecision;
};

export type Ask = {
	kind: 'completion' | 'decision' | 'embedding';
	model: string;
	authorization: string;
	schemaName?: string;
	offeredToolNames?: string[];
	questionNames?: string[];
	about?: string;
	answeredWith?: string;
	refusal?: string;
};

export type Leftovers = { refusals: string[]; unconsumed: Record<string, number> };

export type AModelNobodyPaysFor = {
	url: string;
	decideTurn: (outcome: TurnOutcome) => Promise<void>;
	answerNext: (schemaName: string, document: Record<string, unknown>) => Promise<void>;
	callNext: (toolName: string, argumentsDocument: Record<string, unknown>) => Promise<void>;
	asked: () => Promise<Ask[]>;
	leftovers: () => Promise<Leftovers>;
	reset: () => Promise<void>;
	stop: () => void;
};

export const turnRouterSchemaName = 'bluecollar_turn_router';

export const turnWordsOwingOnlyTheReply = { expectedResults: [] };

export const expectedChangesSchemaName = 'bluecollar_expected_changes';

export const changingNothingTheCheckCanRead = { expectedChanges: [] };

export function addressedToTheAgent(): AddressingDecision {
	return { target: 'bot', shouldRespond: true };
}

export function aTurnStartingWork(message: string, toolNames: string[]): TurnOutcome {
	return {
		message,
		addressing: addressedToTheAgent(),
		turnDecision: {
			route: 'start_task',
			classification: 'bounded_task',
			taskShape: 'maintenance_task',
			level: 'low',
			responseLanguage: 'ko',
			initialToolNames: toolNames,
			isExternalSendRequested: toolNames.includes('message_send')
		}
	};
}

export function aTurnApprovingTheHeldCall(message: string): TurnOutcome {
	return {
		message,
		addressing: addressedToTheAgent(),
		turnDecision: {
			route: 'continue_task',
			classification: 'bounded_task',
			taskShape: 'maintenance_task',
			level: 'low',
			responseLanguage: 'ko',
			expectedToolCount: 'none',
			approval: 'approve'
		}
	};
}

export function aPlanThatNeedsNoClarification(recipientName: string): Record<string, unknown> {
	return {
		summary: `${recipientName}에게 메시지를 보낸다`,
		targets: [recipientName],
		schedule: '',
		startAt: '',
		endAt: '',
		cadence: '',
		externalSend: true,
		thirdPartyExternalSend: false,
		repeated: false,
		highFrequency: false,
		destructive: false,
		permissionChange: false,
		publicDeploy: false,
		paidAction: false,
		requesterAuthorization: 'explicit',
		missingInformation: [],
		continuationInstruction: ''
	};
}

export function replyingAndFinishing(message: string): Record<string, unknown> {
	return { message, final: true, goalStatus: 'satisfied', goalSatisfied: true, completionEvidenceIDs: [] };
}

async function send(url: string, document: unknown): Promise<void> {
	const answer = await fetch(url, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(document)
	});
	if (answer.status !== 204) {
		throw new Error(`the model stand-in refused a script: ${answer.status} ${await answer.text()}`);
	}
}

async function read<Document>(url: string): Promise<Document> {
	const answer = await fetch(url);
	if (!answer.ok) throw new Error(`the model stand-in answered ${url} with ${answer.status}`);
	return (await answer.json()) as Document;
}

async function untilListening(origin: string, child: Bun.Subprocess): Promise<void> {
	for (let attempt = 0; attempt < 120; attempt += 1) {
		const answer = await fetch(`${origin}/leftovers`).catch(() => null);
		if (answer?.ok) return;
		if (child.exitCode !== null) throw new Error(`blueclaw-model-stand-in exited with code ${child.exitCode}`);
		await Bun.sleep(250);
	}
	throw new Error('blueclaw-model-stand-in never started listening');
}

export async function aModelNobodyPaysFor(
	binaryPath: string,
	port: number,
	logPath: string
): Promise<AModelNobodyPaysFor> {
	const origin = `http://127.0.0.1:${port}`;
	const log = openSync(logPath, 'a');
	const child = Bun.spawn([binaryPath, '-listen', `127.0.0.1:${port}`], { stdout: log, stderr: log });
	await untilListening(origin, child);
	return {
		url: `${origin}/v1`,
		decideTurn: (outcome) => send(`${origin}/script/turn`, outcome),
		answerNext: (schemaName, document) => send(`${origin}/script/structured`, { schemaName, document }),
		callNext: (toolName, argumentsDocument) =>
			send(`${origin}/script/action`, { toolName, arguments: argumentsDocument }),
		asked: () => read<Ask[]>(`${origin}/asked`),
		leftovers: () => read<Leftovers>(`${origin}/leftovers`),
		reset: () => send(`${origin}/script/reset`, {}),
		stop: () => child.kill('SIGKILL')
	};
}

function hasUnconsumedScripts(leftovers: Leftovers): boolean {
	return Object.keys(leftovers.unconsumed).length > 0;
}

async function leftoversOnceDrained(model: AModelNobodyPaysFor, seconds: number): Promise<Leftovers> {
	let leftovers = await model.leftovers();
	for (let attempt = 0; attempt < seconds * 4 && hasUnconsumedScripts(leftovers); attempt += 1) {
		await Bun.sleep(250);
		leftovers = await model.leftovers();
	}
	return leftovers;
}

export async function everyScriptWasAskedAndNothingElse(model: AModelNobodyPaysFor, seconds = 60): Promise<void> {
	const leftovers = await leftoversOnceDrained(model, seconds);
	await model.reset();
	if (leftovers.refusals.length === 0 && !hasUnconsumedScripts(leftovers)) return;
	throw new Error(
		`the model stand-in was not asked exactly what the scenario scripted.\n` +
			`  refused: ${JSON.stringify(leftovers.refusals, null, 2)}\n` +
			`  never asked for: ${JSON.stringify(leftovers.unconsumed)}\n` +
			`  asked, in order: ${await whatTheModelWasAsked(model)}`
	);
}

export async function whatTheModelWasAsked(model: AModelNobodyPaysFor): Promise<string> {
	const asked = await model.asked();
	return JSON.stringify(
		asked
			.filter((ask) => ask.kind !== 'embedding')
			.map((ask) => ({
				asked: `${ask.kind} ${ask.schemaName ?? ''}`.trim(),
				about: ask.about,
				answered: ask.refusal ? `refused: ${ask.refusal}` : ask.answeredWith
			})),
		null,
		2
	);
}
