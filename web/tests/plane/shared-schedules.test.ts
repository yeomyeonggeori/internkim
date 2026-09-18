import { afterAll, beforeAll, expect, test } from 'bun:test';
import { SQL } from 'bun';
import { z } from 'zod';
import { aCompanyPlane, type ACompanyPlane } from './a-company-plane';

const scheduleListAnswerSchema = z.object({
	outcome: z.string(),
	result: z.object({
		schedules: z.array(
			z.object({
				scheduleID: z.string(),
				taskInstruction: z.string(),
				description: z.string().optional(),
				cadence: z.string(),
				cronExpression: z.string().optional(),
				status: z.string(),
				nextRunAt: z.string().optional()
			})
		)
	})
});

type ScheduleListAnswer = z.infer<typeof scheduleListAnswerSchema>;

let plane: ACompanyPlane;
let database: SQL;

beforeAll(async () => {
	plane = await aCompanyPlane();
	database = new SQL(plane.blueclawDatabaseURL);
}, 120_000);

afterAll(async () => {
	await database?.close();
	await plane?.stop();
});

async function seedSchedule(
	scheduleID: string,
	creatorPersonID: string,
	description: string,
	taskInstruction: string
): Promise<void> {
	const nextRunAt = new Date(Date.now() + 24 * 60 * 60 * 1000);
	await database`
		INSERT INTO task_schedule (
			task_schedule_id, creator_person_id, name, prompt, execution_mode,
			agent_profile_name, schedule_kind, cron_expression, next_run_at,
			created_at, updated_at, platform, delivery_conversation_id,
			reply_target_id, time_zone, failure_count, last_error,
			next_attempt_at, completed_run_count
		) VALUES (
			${scheduleID}, ${creatorPersonID}, ${description}, ${taskInstruction}, 'agent',
			'', 'cron', '15 9 * * 1-5', ${nextRunAt}, now(), now(), 'buzz', '', '',
			'Asia/Seoul', 0, '', now(), 0
		)
	`;
}

// What the poller writes when a schedule starts a run: the creator, the
// schedule's own conversation namespace, and the schedule's instruction.
async function seedTaskRun(
	taskRunID: string,
	requesterPersonID: string,
	originConversationID: string
): Promise<void> {
	await database`
		INSERT INTO task_run (
			task_run_id, requester_person_id, origin_conversation_id, origin_is_thread,
			current_agent_profile_name, status, prompt, created_at, updated_at
		) VALUES (
			${taskRunID}, ${requesterPersonID}, ${originConversationID}, false,
			'', 'running', 'Post the delivery plan for the week to the project channel.', now(), now()
		)
	`;
}

async function listSchedules(requesterEmail: string): Promise<ScheduleListAnswer> {
	const answer = await fetch('http://plane/v1/tools/schedule_list/invoke', {
		unix: plane.capabilitySocketPath,
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			input: { status: 'active', limit: 20 },
			context: { requesterEmail }
		})
	});
	expect(answer.status, `the plane refused: ${await answer.clone().text()}`).toBe(200);
	return scheduleListAnswerSchema.parse(await answer.json());
}

test('schedule_list returns the full instruction only to its creator', async () => {
	const [requester, colleague] = plane.people;
	const requesterInstruction =
		'Open the weekly operations report, preserve every unresolved item and owner, verify each dependency against the current delivery plan, include the source links and exact due dates, then send the complete summary to the project channel without shortening any section.';
	const colleagueInstruction =
		'Review the private hiring pipeline and send the complete candidate notes to the recruiting channel.';
	expect(requesterInstruction.length).toBeGreaterThan(160);
	await seedSchedule('plane-requester-schedule', requester.memberID, 'Operations follow-up', requesterInstruction);
	await seedSchedule('plane-colleague-schedule', colleague.memberID, 'Recruiting follow-up', colleagueInstruction);

	const requesterAnswer = await listSchedules(requester.email);
	const colleagueAnswer = await listSchedules(colleague.email);

	expect(requesterAnswer.outcome).toBe('succeeded');
	expect(requesterAnswer.result.schedules).toEqual([
		{
			scheduleID: 'plane-requester-schedule',
			taskInstruction: requesterInstruction,
			description: 'Operations follow-up',
			cadence: 'cron',
			cronExpression: '15 9 * * 1-5',
			status: 'active',
			nextRunAt: expect.any(String)
		}
	]);
	expect(colleagueAnswer.result.schedules.map((schedule) => schedule.scheduleID)).toEqual([
		'plane-colleague-schedule'
	]);
	expect(JSON.stringify(requesterAnswer)).not.toContain(colleagueInstruction);
	expect(JSON.stringify(colleagueAnswer)).not.toContain(requesterInstruction);
});

const scheduleWriteAnswerSchema = z.object({
	outcome: z.string(),
	result: z.object({
		scheduleID: z.string(),
		description: z.string(),
		taskInstruction: z.string(),
		timeZone: z.string(),
		kind: z.string(),
		cronExpression: z.string().optional(),
		intervalSecond: z.number().optional(),
		nextRunAt: z.string(),
		conversationID: z.string(),
		replyTargetID: z.string(),
		agentProfileName: z.string()
	})
});

const scheduleCancelAnswerSchema = z.object({
	outcome: z.string(),
	result: z.object({
		cancelled: z.array(z.object({ scheduleID: z.string(), description: z.string() }))
	})
});

type ScheduleCall = {
	toolName: string;
	input: Record<string, unknown>;
	requesterEmail: string;
	requesterPersonID?: string;
	taskRunID?: string;
	approved?: boolean;
	outcome?: string;
};

async function invokeScheduleTool(call: ScheduleCall): Promise<Response> {
	return fetch(`http://plane/v1/tools/${call.toolName}/invoke`, {
		unix: plane.capabilitySocketPath,
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			input: call.input,
			context: {
				requesterEmail: call.requesterEmail,
				requesterPersonID: call.requesterPersonID ?? '',
				taskRunID: call.taskRunID ?? '',
				platform: plane.messengerPlatform,
				conversationID: 'plane-schedule-conversation',
				conversationType: 'channel',
				channelID: 'plane-schedule-conversation',
				replyTargetID: 'plane-schedule-message',
				isApprovalContinuation: call.approved === true
			}
		})
	});
}

async function answerOf(call: ScheduleCall): Promise<unknown> {
	const answer = await invokeScheduleTool(call);
	const written = await answer.clone().text();
	expect(answer.status, `the plane refused ${call.toolName}: ${written}`).toBe(200);
	const carried: unknown = await answer.json();
	const reached = z.object({ outcome: z.string() }).parse(carried);
	expect(reached.outcome, `${call.toolName} answered: ${written}`).toBe(call.outcome ?? 'succeeded');
	return carried;
}

test('a schedule is created, changed and cancelled through the shared catalog', async () => {
	const [requester] = plane.people;

	const created = scheduleWriteAnswerSchema.parse(
		await answerOf({
			toolName: 'schedule_create',
			requesterEmail: requester.email,
			input: {
				taskInstruction: 'Post the delivery plan for the week to the project channel.',
				description: 'Weekly delivery plan',
				kind: 'cron',
				cronExpression: '0 9 * * 1',
				timeZone: 'Asia/Seoul',
				repeatPolicy: 'unbounded'
			}
		})
	);
	expect(created.outcome).toBe('succeeded');
	expect(created.result.conversationID).toBe('plane-schedule-conversation');
	expect(created.result.replyTargetID).toBe('plane-schedule-message');

	const listedAfterCreate = await listSchedules(requester.email);
	expect(
		listedAfterCreate.result.schedules.map((schedule) => schedule.scheduleID)
	).toContain(created.result.scheduleID);

	const updated = scheduleWriteAnswerSchema.parse(
		await answerOf({
			toolName: 'schedule_update',
			requesterEmail: requester.email,
			input: {
				scheduleHint: 'Weekly delivery plan',
				kind: 'interval',
				intervalSecond: 3600,
				repeatPolicy: 'unbounded'
			}
		})
	);
	expect(updated.result.scheduleID).toBe(created.result.scheduleID);
	expect(updated.result.kind).toBe('interval');
	expect(updated.result.intervalSecond).toBe(3600);

	const refused = await answerOf({
		toolName: 'schedule_cancel',
		requesterEmail: requester.email,
		outcome: 'denied',
		input: { scheduleHints: ['Weekly delivery plan'] }
	});
	expect(JSON.stringify(refused)).toContain('approval_required');

	const cancelled = scheduleCancelAnswerSchema.parse(
		await answerOf({
			toolName: 'schedule_cancel',
			requesterEmail: requester.email,
			requesterPersonID: requester.memberID,
			approved: true,
			input: { scheduleHints: ['Weekly delivery plan'] }
		})
	);
	expect(cancelled.result.cancelled).toEqual([
		{ scheduleID: created.result.scheduleID, description: 'Weekly delivery plan' }
	]);

	const listedAfterCancel = await listSchedules(requester.email);
	expect(
		listedAfterCancel.result.schedules.map((schedule) => schedule.scheduleID)
	).not.toContain(created.result.scheduleID);
});

const scheduleRefusalSchema = z.object({
	outcome: z.string(),
	errorCode: z.string(),
	retryable: z.boolean().optional(),
	message: z.string()
});

test('a schedule that a schedule started writes no schedule', async () => {
	const [requester] = plane.people;
	await seedSchedule(
		'plane-amplifying-schedule',
		requester.memberID,
		'Amplifying follow-up',
		'Post the delivery plan for the week to the project channel.'
	);
	await seedTaskRun(
		'plane-scheduled-run',
		requester.memberID,
		'schedule:plane-amplifying-schedule'
	);
	await seedTaskRun('plane-conversation-run', requester.memberID, 'plane-schedule-conversation');

	const refusedCreate = scheduleRefusalSchema.parse(
		await answerOf({
			toolName: 'schedule_create',
			requesterEmail: requester.email,
			taskRunID: 'plane-scheduled-run',
			outcome: 'failed',
			input: {
				taskInstruction: 'Post the delivery plan again, every hour.',
				description: 'Amplified plan',
				kind: 'interval',
				intervalSecond: 3600,
				timeZone: 'Asia/Seoul',
				repeatPolicy: 'unbounded'
			}
		})
	);
	expect(refusedCreate.errorCode).toBe('not_allowed');
	expect(refusedCreate.retryable ?? false).toBe(false);

	const refusedUpdate = scheduleRefusalSchema.parse(
		await answerOf({
			toolName: 'schedule_update',
			requesterEmail: requester.email,
			taskRunID: 'plane-scheduled-run',
			outcome: 'failed',
			input: {
				scheduleHint: 'Amplifying follow-up',
				intervalSecond: 60,
				repeatPolicy: 'unbounded'
			}
		})
	);
	expect(refusedUpdate.errorCode).toBe('not_allowed');

	const listedAfterRefusal = await listSchedules(requester.email);
	expect(listedAfterRefusal.result.schedules.map((schedule) => schedule.description)).not.toContain(
		'Amplified plan'
	);
	expect(
		listedAfterRefusal.result.schedules.find(
			(schedule) => schedule.scheduleID === 'plane-amplifying-schedule'
		)?.cadence
	).toBe('cron');

	const changed = scheduleWriteAnswerSchema.parse(
		await answerOf({
			toolName: 'schedule_update',
			requesterEmail: requester.email,
			taskRunID: 'plane-conversation-run',
			input: {
				scheduleHint: 'Amplifying follow-up',
				kind: 'interval',
				intervalSecond: 7200,
				repeatPolicy: 'unbounded'
			}
		})
	);
	expect(changed.result.scheduleID).toBe('plane-amplifying-schedule');
	expect(changed.result.intervalSecond).toBe(7200);
});
