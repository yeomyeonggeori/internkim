import { expect } from 'bun:test';
import { type ACompanyPlane, hostServiceHeaders } from './a-company-plane';
import {
	aTurnAnsweringTheParkedQuestion,
	aTurnStartingWork,
	changingNothingTheCheckCanRead,
	expectedChangesSchemaName,
	replyingAndFinishing,
	turnRouterSchemaName,
	turnWordsOwingOnlyTheReply,
	whatTheModelWasAsked
} from './a-model-nobody-pays-for';
import { directMessagesDelivered, messagesPostedTo } from './a-messenger-nobody-runs';
import { handToTheRelay, until } from './an-inbound-message';

export type AParkedQuestion = {
	conversationID: string;
	marker: string;
	interruption: () => Promise<void>;
};

const answer = '응 보내줘';
const closingReply = '보냈습니다';

type LedgerEvent = { name: string; body: string };
type TaskRunOnTheLedger = { status: string; failureReason?: string; events: LedgerEvent[] };

export async function aQuestionParkedAcrossAnInterruptionIsAskedAnsweredAndCarriedOutOnce(
	plane: ACompanyPlane,
	parked: AParkedQuestion
): Promise<void> {
	const [sender, recipient] = plane.people;
	const request = `${recipient.name}한테 DM으로 "${parked.marker}" 보내줘`;
	await scriptTheModel(plane, request, { targetType: 'directMessage', personHint: recipient.name, message: parked.marker });

	const asked = await handToTheRelay(plane, {
		sender: { email: sender.email, name: sender.name },
		conversationID: parked.conversationID,
		messageID: 'message-1',
		message: request
	});
	expect(asked.status, `the relay refused the turn: ${await asked.clone().text()}`).toBe(202);
	await waitingFor(plane, parked, 'the requester was never asked whether the message may be sent', () => postsTo(plane, parked).length > 0);
	const theQuestion = postsTo(plane, parked)[0];
	await waitingFor(plane, parked, 'the run never parked on its question', async () =>
		(await theTaskRuns(plane)).some((taskRun) => taskRun.status === 'waiting_approval')
	);

	await parked.interruption();

	const answering = await handToTheRelay(plane, {
		sender: { email: sender.email },
		conversationID: parked.conversationID,
		messageID: 'message-2',
		message: answer
	});
	expect(answering.status, await answering.clone().text()).toBe(202);
	await waitingFor(plane, parked, 'the closing reply never reached the conversation after the interruption', () =>
		postsTo(plane, parked).includes(closingReply)
	);

	const whatHappened = await whatTheRunSaw(plane, parked);
	expect(postsTo(plane, parked), `the conversation did not get the question once and the reply once.\n${whatHappened}`).toEqual([
		theQuestion,
		closingReply
	]);
	expect(messagesCarrying(plane, parked.marker), `the approved message was not sent exactly once.\n${whatHappened}`).toBe(1);
	const taskRuns = await theTaskRuns(plane);
	expect(taskRuns.map((taskRun) => taskRun.status), `the answer did not resume the one parked run.\n${whatHappened}`).toEqual(['completed']);
	expect(countsOf(taskRuns[0], ['approval.hold_opened', 'approval.decided', 'approval.hold_spent']), whatHappened).toEqual({
		'approval.hold_opened': 1,
		'approval.decided': 1,
		'approval.hold_spent': 1
	});
}

async function scriptTheModel(plane: ACompanyPlane, request: string, messageToSend: Record<string, unknown>): Promise<void> {
	await plane.model.decideTurn(aTurnStartingWork(request, ['message_send']));
	await plane.model.decideTurn(aTurnAnsweringTheParkedQuestion(answer));
	await plane.model.answerNext(turnRouterSchemaName, turnWordsOwingOnlyTheReply);
	await plane.model.answerNext(expectedChangesSchemaName, changingNothingTheCheckCanRead);
	await plane.model.answerNext(expectedChangesSchemaName, changingNothingTheCheckCanRead);
	await plane.model.callNext('message_send', messageToSend);
	await plane.model.callNext('reply', replyingAndFinishing(closingReply));
}

function postsTo(plane: ACompanyPlane, parked: AParkedQuestion): string[] {
	return messagesPostedTo(plane.connector, parked.conversationID);
}

function messagesCarrying(plane: ACompanyPlane, marker: string): number {
	return directMessagesDelivered(plane.connector).filter((call) => JSON.stringify(call.body ?? '').includes(marker)).length;
}

function countsOf(taskRun: TaskRunOnTheLedger, eventNames: string[]): Record<string, number> {
	return Object.fromEntries(eventNames.map((name) => [name, taskRun.events.filter((event) => event.name === name).length]));
}

async function waitingFor(
	plane: ACompanyPlane,
	parked: AParkedQuestion,
	what: string,
	isReady: () => boolean | Promise<boolean>,
	seconds = 60
): Promise<void> {
	try {
		await until(what, isReady, seconds);
	} catch {
		throw new Error(`${what}\n${await whatTheRunSaw(plane, parked)}`);
	}
}

async function whatTheRunSaw(plane: ACompanyPlane, parked: AParkedQuestion): Promise<string> {
	return (
		`  posted to the conversation: ${JSON.stringify(postsTo(plane, parked), null, 2)}\n` +
		`  the connector saw: ${JSON.stringify(plane.connector.pathsCalled())}\n` +
		`  the model was asked: ${await whatTheModelWasAsked(plane.model)}\n` +
		`  the ledger says: ${describedLedger(await theTaskRuns(plane))}`
	);
}

function describedLedger(taskRuns: TaskRunOnTheLedger[]): string {
	if (!taskRuns.length) return 'no turn opened a task run';
	return taskRuns
		.map((taskRun) => {
			const events = taskRun.events.map((event) =>
				/fail|error|refus|unavailable|result|reject|approval|hold|task_launched/.test(event.name)
					? `${event.name}(${event.body.slice(0, 400)})`
					: event.name
			);
			return `${taskRun.status} ${taskRun.failureReason ?? ''}\n    ${events.join('\n    ')}`;
		})
		.join('\n  ');
}

async function theTaskRuns(plane: ACompanyPlane): Promise<TaskRunOnTheLedger[]> {
	const headers = hostServiceHeaders(plane.serviceToken);
	const listing = await fetch(`${plane.blueclawURL}/admin/api/run`, { headers }).catch(() => null);
	if (!listing?.ok) return [];
	const listed: { taskRunID: string; status: string; failureReason?: string }[] = await listing.json();
	return Promise.all(
		listed.map(async (taskRun) => {
			const detail = await fetch(`${plane.blueclawURL}/admin/api/run/detail?taskRunID=${encodeURIComponent(taskRun.taskRunID)}`, {
				headers
			}).catch(() => null);
			const document: { taskEvents?: LedgerEvent[] } = detail?.ok ? await detail.json() : {};
			return { status: taskRun.status, failureReason: taskRun.failureReason, events: document.taskEvents ?? [] };
		})
	);
}
