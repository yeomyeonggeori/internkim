import { afterAll, afterEach, beforeAll, expect, test } from 'bun:test';
import { aCompanyPlane, type ACompanyPlane } from './a-company-plane';
import {
	aTurnStartingWork,
	changingNothingTheCheckCanRead,
	everyScriptWasAskedAndNothingElse,
	expectedChangesSchemaName,
	replyingAndFinishing,
	turnRouterSchemaName,
	turnWordsOwingOnlyTheReply,
	whatTheModelWasAsked
} from './a-model-nobody-pays-for';
import { messagesPostedTo } from './a-messenger-nobody-runs';
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

beforeAll(async () => {
	plane = await aCompanyPlane({ messengerPlatform: 'buzz', inbound: 'acp' });
}, 180_000);

afterEach(async () => {
	await everyScriptWasAskedAndNothingElse(plane.model);
}, 90_000);

afterAll(async () => {
	await plane?.stop();
});

test('a message written in a room and addressed to the agent is answered in that room', async () => {
	const [sender] = plane.people;
	const request = '인턴킴, 이번 주 정산 어떻게 됐는지 한 줄로 알려줘';
	await plane.model.decideTurn(aTurnStartingWork(request, []));
	await plane.model.answerNext(turnRouterSchemaName, turnWordsOwingOnlyTheReply);
	await plane.model.answerNext(expectedChangesSchemaName, changingNothingTheCheckCanRead);
	await plane.model.callNext('reply', replyingAndFinishing(marker));

	const asked = await handToTheRelay(plane, {
		sender: { email: sender.email, name: sender.name },
		conversationID: theRoom(),
		messageID: 'message-room-1',
		conversationType: 'channel',
		botMentioned: true,
		channelName: '공지',
		message: request
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
	const overheardWords = '오늘 점심 뭐 먹지';
	await plane.model.decideTurn({ message: overheardWords, addressing: { target: 'anyone', shouldRespond: false } });
	const postsBeforeItArrived = postsToTheRoom().length;
	const gatesBeforeItArrived = await addressingGatesAsked();

	const overheard = await handToTheRelay(plane, {
		sender: { email: sender.email, name: sender.name },
		conversationID: theRoom(),
		messageID: 'message-room-2',
		conversationType: 'channel',
		botMentioned: false,
		channelName: '공지',
		message: overheardWords
	});
	expect(overheard.status, await overheard.clone().text()).toBe(202);

	// The gate having been asked is the proof the message was read; waiting a fixed
	// number of seconds instead would only prove the machine was slow.
	await waitingFor(
		'the addressing gate was never asked about the message nobody addressed',
		async () => (await addressingGatesAsked()) > gatesBeforeItArrived,
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
	return messagesPostedTo(plane.connector, theRoom());
}

async function addressingGatesAsked(): Promise<number> {
	const asked = await plane.model.asked();
	return asked.filter(
		(ask) => ask.kind === 'decision' && (ask.questionNames ?? []).some((name) => name.endsWith('.target'))
	).length;
}

// until() is handed its sentence before it starts waiting, so what the run
// actually did is gathered here, once it is known that it did not happen.
async function waitingFor(
	what: string,
	ready: () => boolean | Promise<boolean>,
	seconds = 60
): Promise<void> {
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
		`  the model was asked: ${await whatTheModelWasAsked(plane.model)}\n` +
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
