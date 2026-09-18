import { afterAll, beforeAll, expect, test } from 'bun:test';
import { aCompanyPlane, type ACompanyPlane } from './a-company-plane';
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

function sendingTheMessage(recipientName: string): string {
	return JSON.stringify({
		targetType: 'directMessage',
		personHint: recipientName,
		message: marker
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

// The pre-turn plan gate fires because message_send is a send tool, and it is a
// different question from the one under test. This plan asks nothing, so the
// only question the requester gets is the tool call gate's.
function aPlanThatNeedsNoClarification(recipientName: string): string {
	return JSON.stringify({
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
	});
}

function aSatisfiedCompletionJudge(): string {
	return JSON.stringify({ satisfied: true, missingWork: [], reason: '보낸 기록이 남았습니다' });
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
		initialToolNames: ['message_send'],
		priorTaskReference: 'none',
		...fields
	});
}

// The restarted daemon reads the person's words itself, which is why the relay
// keeps the words rather than the option it thinks they chose.
// The router schema for a turn that answers a pending question caps
// initialToolNames at zero, and bluecollar refuses a document over that cap.
function aRouterReadingTheAnswerAsApproval(): string {
	return aRouterDocument({ route: 'continue_task', approval: 'approve', initialToolNames: [] });
}

beforeAll(async () => {
	plane = await aCompanyPlane({ messengerPlatform: 'buzz', inbound: 'acp' });
}, 180_000);

afterAll(async () => {
	await plane?.stop();
});

test('a question held across a blueclaw restart is asked once and answered once', async () => {
	const [sender, recipient] = plane.people;
	plane.model.answerNext('bluecollar_turn_router', aRouterDocument({}));
	plane.model.answerNext('bluecollar_turn_router', aRouterReadingTheAnswerAsApproval());
	plane.model.answerNext('bluecollar_execution_plan', aPlanThatNeedsNoClarification(recipient.name));
	plane.model.callNext('message_send', sendingTheMessage(recipient.name));
	plane.model.callNext('finish', finishing('보냈습니다'));
	plane.model.answerNext('bluecollar_completion_judge', aSatisfiedCompletionJudge());

	const asked = await handToTheRelay(plane, {
		sender: { email: sender.email, name: sender.name },
		conversationID: theConversation(),
		messageID: 'message-1',
		message: `${recipient.name}한테 DM으로 "${marker}" 보내줘`
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
		message: '응 보내줘'
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
