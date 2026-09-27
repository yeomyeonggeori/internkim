import { afterAll, afterEach, beforeAll, expect, test } from 'bun:test';
import { aCompanyPlane, type ACompanyPlane } from './a-company-plane';
import {
	aPlanThatNeedsNoClarification,
	aTurnApprovingTheHeldCall,
	aTurnStartingWork,
	changingNothingTheCheckCanRead,
	everyScriptWasAskedAndNothingElse,
	expectedChangesSchemaName,
	replyingAndFinishing,
	turnRouterSchemaName,
	turnWordsOwingOnlyTheReply,
	whatTheModelWasAsked
} from './a-model-nobody-pays-for';
import { directMessagesDelivered } from './a-messenger-nobody-runs';
import { handToTheRelay, until } from './an-inbound-message';

// 이샘플 asks the agent to write to 박예시, the agent stops to ask whether it may,
// and the daemon holding that question dies before anybody answers. It comes
// back, loads the session, and asks about the same held call again — and the
// relay, which kept the answer nobody had given yet, must put the question to
// 이샘플 once and only once.

let plane: ACompanyPlane;

const marker = `평면 재시작 점검 ${Date.now()}`;

function theConversation(): string {
	return `conversation-restart-${plane.runIdentifier}`;
}

function sendingTheMessage(recipientName: string): Record<string, unknown> {
	return {
		targetType: 'directMessage',
		personHint: recipientName,
		message: marker
	};
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

test('a question held across a blueclaw restart is asked once and answered once', async () => {
	const [sender, recipient] = plane.people;
	const request = `${recipient.name}한테 DM으로 "${marker}" 보내줘`;
	const answer = '응 보내줘';
	await plane.model.decideTurn(aTurnStartingWork(request, ['message_send']));
	await plane.model.decideTurn(aTurnApprovingTheHeldCall(answer));
	await plane.model.answerNext(turnRouterSchemaName, turnWordsOwingOnlyTheReply);
	await plane.model.answerNext(turnRouterSchemaName, turnWordsOwingOnlyTheReply);
	await plane.model.answerNext('bluecollar_execution_plan', aPlanThatNeedsNoClarification(recipient.name));
	await plane.model.answerNext(expectedChangesSchemaName, changingNothingTheCheckCanRead);
	await plane.model.answerNext(expectedChangesSchemaName, changingNothingTheCheckCanRead);
	await plane.model.callNext('message_send', sendingTheMessage(recipient.name));
	await plane.model.callNext('reply', replyingAndFinishing('보냈습니다'));

	const asked = await handToTheRelay(plane, {
		sender: { email: sender.email, name: sender.name },
		conversationID: theConversation(),
		messageID: 'message-1',
		message: request
	});
	expect(asked.status, `the relay refused the turn: ${await asked.clone().text()}`).toBe(202);

	await waitingFor(
		'the requester was never asked whether the message may be sent',
		() => postsToTheConversation().length > 0
	);
	const theQuestion = postsToTheConversation()[0];
	const timesAskedBeforeTheRestart = postsToTheConversation().filter(
		(message) => message === theQuestion
	).length;

	// Nobody has answered yet, and the daemon holding the question goes away.
	await plane.restartBlueclaw();

	const answering = await handToTheRelay(plane, {
		sender: { email: sender.email },
		conversationID: theConversation(),
		messageID: 'message-2',
		message: answer
	});
	expect(answering.status, await answering.clone().text()).toBe(202);

	// The relay reconnects, loads the session, and the restarted daemon asks about
	// the held call again, so the answer travels a longer way here than it does in
	// a run nothing interrupted.
	await waitingFor(
		'the message never reached the messenger connector after the restart',
		() =>
			directMessagesDelivered(plane.connector).some((call) =>
				JSON.stringify(call.body ?? '').includes(marker)
			)
	);

	const timesAsked = postsToTheConversation().filter((message) => message === theQuestion).length;
	expect(
		timesAsked,
		`이샘플 was asked ${timesAsked} times for the one answer they gave.\n` +
			`  posted to the conversation: ${JSON.stringify(postsToTheConversation(), null, 2)}\n` +
			`  the connector saw: ${JSON.stringify(plane.connector.calls, null, 2)}`
	).toBe(timesAskedBeforeTheRestart);
}, 180_000);

function postsToTheConversation(): string[] {
	const posted: string[] = [];
	for (const call of plane.connector.calls) {
		if (!call.path.endsWith('/message.post')) continue;
		if (typeof call.body !== 'object' || call.body === null) continue;
		const document = call.body as Record<string, unknown>;
		if (document.channelID !== theConversation()) continue;
		posted.push(typeof document.message === 'string' ? document.message : '');
	}
	return posted;
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
		`  posted to the conversation: ${JSON.stringify(postsToTheConversation(), null, 2)}\n` +
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
