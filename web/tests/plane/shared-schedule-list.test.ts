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
