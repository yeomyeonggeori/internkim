import { readFileSync } from 'node:fs';
import { describe, expect, test } from 'bun:test';
import {
	approvalDecisionRequestOf,
	approvalDecisions,
	askRequestedEventName,
	confirmationRequestedEventName,
	pendingApprovalOf,
	readApprovalOutcome,
	waitingApprovalStatus,
	type TaskDetail
} from '../../../src/routes/runs/runs-api';

function relayAnswerForARunWaitingOnApproval(): TaskDetail {
	return {
		taskRun: {
			taskRunID: 'run-77',
			status: waitingApprovalStatus,
			requesterPersonID: 'person-1',
			requesterDisplayName: '이샘플',
			prompt: '9월 워크숍 예약을 확정해줘'
		},
		taskEvents: [
			{ name: 'agent.goal.waiting_approval', body: '{}' },
			{ name: askRequestedEventName, body: JSON.stringify({ approvalScope: 'calendar' }) },
			{
				name: confirmationRequestedEventName,
				body: JSON.stringify({ userFacingMessage: 'Book the room for 30 people?' })
			}
		]
	};
}

describe('a run waiting on approval, read out of a relay answer', () => {
	test('carries the question and the scope the agent asked for', () => {
		const approval = pendingApprovalOf(relayAnswerForARunWaitingOnApproval());

		expect(approval?.taskRun.taskRunID).toBe('run-77');
		expect(approval?.question).toBe('Book the room for 30 people?');
		expect(approval?.scope).toBe('calendar');
	});

	test('the latest question wins when the agent asked more than once', () => {
		const detail = relayAnswerForARunWaitingOnApproval();
		detail.taskEvents.push({
			name: confirmationRequestedEventName,
			body: JSON.stringify({ userFacingMessage: 'Book the larger room instead?' })
		});

		expect(pendingApprovalOf(detail)?.question).toBe('Book the larger room instead?');
	});

	test('a run in any other status is not a pending approval', () => {
		const detail = relayAnswerForARunWaitingOnApproval();
		detail.taskRun.status = 'running';

		expect(pendingApprovalOf(detail)).toBeUndefined();
	});

	test('an unparseable event body leaves the question empty rather than throwing', () => {
		const detail = relayAnswerForARunWaitingOnApproval();
		detail.taskEvents = [{ name: confirmationRequestedEventName, body: 'not json' }];

		expect(pendingApprovalOf(detail)).toEqual({ taskRun: detail.taskRun, question: '', scope: '' });
	});
});

describe('the decision the approvals page sends', () => {
	test('names the run and the decision, and nothing else', () => {
		expect(approvalDecisionRequestOf('run-77', 'confirm')).toEqual({
			taskRunID: 'run-77',
			decision: 'confirm'
		});
	});

	test('offers the three decisions blueclaw accepts', () => {
		const decisionsBlueclawAccepts = readFileSync(
			'../.dependency/blueclaw/internal/adminapi/task_approval_handler.go',
			'utf8'
		)
			.split('func approvalTurnDecision')[1]
			.split('default:')[0]
			.match(/case "([a-z_]+)":/g)
			?.map((line) => line.replace(/case "|":/g, ''));

		expect([...approvalDecisions].map(String)).toEqual(decisionsBlueclawAccepts ?? []);
	});

	test('reads the status back out of the relay answer', () => {
		expect(readApprovalOutcome({ taskRunID: 'run-77', status: 'running' }, 'run-77')).toEqual({
			taskRunID: 'run-77',
			status: 'running'
		});
	});

	test('keeps the run it asked about when the answer names none', () => {
		expect(readApprovalOutcome(null, 'run-77')).toEqual({ taskRunID: 'run-77', status: '' });
	});
});

describe('the task event names the ledger reads', () => {
	test('are names the generated task-event schema declares', () => {
		const declared = JSON.parse(
			readFileSync('../pkg/capabilityprotocol/generated/json-schema/task-event-name.schema.json', 'utf8')
		) as { anyOf: { enum?: string[] }[] };
		const names = declared.anyOf.flatMap((branch) => branch.enum ?? []);

		expect(names).toContain(confirmationRequestedEventName);
		expect(names).toContain(askRequestedEventName);
	});
});
