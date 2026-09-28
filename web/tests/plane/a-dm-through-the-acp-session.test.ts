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
import { handToTheRelay, until, type AnInboundMessage } from './an-inbound-message';

// 이샘플 writes to the agent and the agent writes to 박예시. The whole way there
// is the ACP session: the relay opens it, prompts it, is asked for 이샘플's
// permission on the messenger, and carries the words they answer with back to
// the agent, which is the only side that reads what they meant.

let plane: ACompanyPlane;

const marker = `평면 acp 점검 ${Date.now()}`;

// The loop offers one native tool per action, named after the action or, for a
// continue, after the tool it would call. Its arguments are that tool's input.
function sendingTheMessage(recipientName: string): Record<string, unknown> {
	return {
		targetType: 'directMessage',
		personHint: recipientName,
		message: marker
	};
}

async function askTheRelay(message: AnInboundMessage): Promise<Response> {
	return handToTheRelay(plane, message);
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

test('a message the relay carries becomes a turn, an approval, and a message in the recipient inbox', async () => {
	const [sender, recipient] = plane.people;
	const request = `${recipient.name}한테 DM으로 "${marker}" 보내줘`;
	const answer = '응 보내줘';
	await plane.model.decideTurn(aTurnStartingWork(request, ['message_send']));
	await plane.model.decideTurn(aTurnApprovingTheHeldCall(answer));
	await plane.model.answerNext(turnRouterSchemaName, turnWordsOwingOnlyTheReply);
	await plane.model.answerNext(turnRouterSchemaName, turnWordsOwingOnlyTheReply);
	await plane.model.answerNext('bluecollar_execution_plan', aPlanThatNeedsNoClarification(recipient.name));
	await plane.model.answerNext(expectedChangesSchemaName, changingNothingTheCheckCanRead);
	await plane.model.callNext('message_send', sendingTheMessage(recipient.name));
	await plane.model.callNext('reply', replyingAndFinishing('보냈습니다'));

	const asked = await askTheRelay({
		sender: { email: sender.email, name: sender.name },
		conversationID: `conversation-${plane.runIdentifier}`,
		messageID: 'message-1',
		message: request
	});
	// The door answers once the event is on disk, so a relay that dies here still
	// owes the message. The turn happens after.
	expect(asked.status, `the relay refused the turn: ${await asked.clone().text()}`).toBe(202);

	// The question the runtime worded reaches the requester as a message before
	// anything is sent, and the person's answer arrives as the next message in
	// that conversation.
	await untilTheQuestionIsAsked();
	const answering = await askTheRelay({
		sender: { email: sender.email },
		conversationID: `conversation-${plane.runIdentifier}`,
		messageID: 'message-2',
		message: answer
	});
	expect(answering.status, await answering.clone().text()).toBe(202);

	await until(
		`nothing reached the messenger connector.\n` +
			`  it saw: ${JSON.stringify(plane.connector.pathsCalled())}`,
		() => directMessagesDelivered(plane.connector).length > 0
	);
	const delivered = directMessagesDelivered(plane.connector);
	expect(
		delivered.length,
		`nothing reached the messenger connector.\n` +
			`  it saw: ${JSON.stringify(plane.connector.pathsCalled())}\n` +
			`  the model was asked: ${await whatTheModelWasAsked(plane.model)}\n` +
			`  the ledger says: ${await theLedger()}`
	).toBeGreaterThan(0);
	expect(
		delivered.some((call) => JSON.stringify(call.body ?? '').includes(marker)),
		'the connector was called but not with the message the agent was asked to send'
	).toBe(true);
}, 180_000);

test('the connector event route refuses a turn while the acp session admits them', async () => {
	const answer = await fetch(`${plane.blueclawURL}/connectors/api/events`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({})
	});
	expect(answer.status, await answer.clone().text()).toBe(409);
	expect(await answer.text()).toContain('-inbound acp');
});

function postsToTheConversation(): { body: unknown }[] {
	return plane.connector.calls.filter((call) => /\/message\.post$/.test(call.path));
}

async function untilTheQuestionIsAsked(): Promise<void> {
	for (let attempt = 0; attempt < 240; attempt += 1) {
		if (postsToTheConversation().length > 0) return;
		await Bun.sleep(250);
	}
	throw new Error(
		`the requester was never asked. the connector saw: ${JSON.stringify(plane.connector.pathsCalled())}`
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
		lines.push(
			`${taskRun.status} ${taskRun.failureReason ?? ''}\n    ${events.join('\n    ')}`
		);
	}
	return lines.join('\n  ');
}
