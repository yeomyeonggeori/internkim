import { afterAll, beforeAll, expect, test } from 'bun:test';
import { aCompanyPlane, type ACompanyPlane } from './a-company-plane';
import { handToTheRelay, until } from './an-inbound-message';

// 이샘플 writes in a room, names the agent, and the agent answers in that room.
// Nothing is sent to anybody else here: the question is only whether a message
// written where several people talk becomes a turn at all, and whether the one
// written without the agent in it stays unanswered.

let plane: ACompanyPlane;

const marker = `평면 방 점검 ${Date.now()}`;

function theRoom(): string {
	return `conversation-room-${plane.runIdentifier}`;
}

// A message in a multi-person conversation is put to the addressing gate before
// anything else, and an unscripted gate answers an empty document, which reads
// as "nobody asked me" and ends the turn. So both stories say which it is.
function addressedToTheAgent(): string {
	return JSON.stringify({
		target: 'agent',
		shouldRespond: true,
		dutyMatch: false,
		dutyName: '',
		dutyConfidence: 0
	});
}

function addressedToNobodyInParticular(): string {
	return JSON.stringify({
		target: 'anyone',
		shouldRespond: false,
		dutyMatch: false,
		dutyName: '',
		dutyConfidence: 0
	});
}

function aRouterDocument(fields: Record<string, unknown>): string {
	return JSON.stringify({
		route: 'start_task',
		classification: 'bounded_task',
		taskShape: 'maintenance_task',
		level: 'low',
		requestedOutputFormats: null,
		expectedResults: [],
		siteRequestEvidence: '',
		responseLanguage: 'ko',
		reason: 'plane scenario',
		userFacingReply: '',
		initialToolNames: [],
		priorTaskReference: 'none',
		...fields
	});
}

function finishing(reply: string): string {
	return JSON.stringify({
		message: reply,
		goalStatus: 'satisfied',
		goalSatisfied: true,
		completionEvidenceIDs: []
	});
}

beforeAll(async () => {
	plane = await aCompanyPlane({ messengerPlatform: 'buzz', inbound: 'acp' });
}, 180_000);

afterAll(async () => {
	await plane?.stop();
});

test('a message written in a room and addressed to the agent is answered in that room', async () => {
	const [sender] = plane.people;
	plane.model.answerNext('bluecollar_addressing_classification', addressedToTheAgent());
	plane.model.answerNext('bluecollar_turn_router', aRouterDocument({}));
	plane.model.callNext('finish', finishing(marker));

	const asked = await handToTheRelay(plane, {
		sender: { email: sender.email, name: sender.name },
		conversationID: theRoom(),
		messageID: 'message-room-1',
		conversationType: 'channel',
		botMentioned: true,
		channelName: '공지',
		message: '인턴킴, 이번 주 정산 어떻게 됐는지 한 줄로 알려줘'
	});
	// The door answers once the event is on disk, so what the agent did with it is
	// read off the connector rather than off this response.
	expect(asked.status, `the relay refused the turn: ${await asked.clone().text()}`).toBe(202);

	await waitingFor(
		'the agent never answered in the room it was written in',
		() => postsToTheRoom().some((message) => message.includes(marker))
	);
}, 120_000);

test('a message in that room that names nobody is answered by nobody', async () => {
	const [sender] = plane.people;
	plane.model.answerNext('bluecollar_addressing_classification', addressedToNobodyInParticular());
	const postsBeforeItArrived = postsToTheRoom().length;
	const gatesBeforeItArrived = addressingGatesAsked();

	const overheard = await handToTheRelay(plane, {
		sender: { email: sender.email, name: sender.name },
		conversationID: theRoom(),
		messageID: 'message-room-2',
		conversationType: 'channel',
		botMentioned: false,
		channelName: '공지',
		message: '오늘 점심 뭐 먹지'
	});
	expect(overheard.status, await overheard.clone().text()).toBe(202);

	// The gate having been asked is the proof the message was read; waiting a fixed
	// number of seconds instead would only prove the machine was slow.
	await waitingFor(
		'the addressing gate was never asked about the message nobody addressed',
		() => addressingGatesAsked() > gatesBeforeItArrived,
		30
	);
	await Bun.sleep(2000);

	expect(
		postsToTheRoom(),
		`the agent wrote in the room without being asked to.\n` +
			`  the room was written: ${JSON.stringify(postsToTheRoom(), null, 2)}\n` +
			`  the connector saw: ${JSON.stringify(plane.connector.pathsCalled())}`
	).toHaveLength(postsBeforeItArrived);
}, 120_000);

// The connector records the whole call, and what a reader wants from a post is
// the words in it.
function postsToTheRoom(): string[] {
	const posted: string[] = [];
	for (const call of plane.connector.calls) {
		if (!call.path.endsWith('/message.post')) continue;
		if (typeof call.body !== 'object' || call.body === null) continue;
		const document = call.body as Record<string, unknown>;
		if (document.channelID !== theRoom()) continue;
		posted.push(typeof document.message === 'string' ? document.message : '');
	}
	return posted;
}

function addressingGatesAsked(): number {
	return plane.model.completions.filter(
		(completion) => completion.toolChoice === 'bluecollar_addressing_classification'
	).length;
}

// until() is handed its sentence before it starts waiting, so what the run
// actually did is gathered here, once it is known that it did not happen.
async function waitingFor(what: string, ready: () => boolean, seconds = 60): Promise<void> {
	try {
		await until(what, ready, seconds);
	} catch {
		throw new Error(`${what}\n${await whatTheRunSaw()}`);
	}
}

async function whatTheRunSaw(): Promise<string> {
	return (
		`  the room was written: ${JSON.stringify(postsToTheRoom(), null, 2)}\n` +
		`  the connector saw: ${JSON.stringify(plane.connector.pathsCalled())}\n` +
		`  the model answered: ${JSON.stringify(plane.model.completions.map((call) => call.answeredWith))}\n` +
		`  the ledger says: ${await theLedger()}`
	);
}

// A status and a tool name send the reader to a log they no longer have; the
// run's own ledger says which step the turn stopped on.
async function theLedger(): Promise<string> {
	const listing = await fetch(`${plane.blueclawURL}/admin/api/run`).catch(
		(unreachable) => new Response(String(unreachable), { status: 599 })
	);
	if (!listing.ok) return `the run list answered ${listing.status}: ${await listing.text()}`;
	const taskRuns = (await listing.json()) as { taskRunID: string; status: string; failureReason?: string }[];
	if (!taskRuns.length) return 'no turn opened a task run';
	const lines: string[] = [];
	for (const taskRun of taskRuns) {
		const detail = await fetch(
			`${plane.blueclawURL}/admin/api/run/detail?taskRunID=${encodeURIComponent(taskRun.taskRunID)}`
		).catch(() => null);
		const document = detail?.ok
			? ((await detail.json()) as { taskEvents?: { name: string; body: string }[] })
			: null;
		const events = (document?.taskEvents ?? []).map((event) =>
			/fail|error|refus|unavailable|result/.test(event.name)
				? `${event.name}(${event.body.slice(0, 400)})`
				: event.name
		);
		lines.push(`${taskRun.status} ${taskRun.failureReason ?? ''}\n    ${events.join('\n    ')}`);
	}
	return lines.join('\n  ');
}
