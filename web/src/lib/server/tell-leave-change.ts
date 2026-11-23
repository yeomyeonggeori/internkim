import type { SupabaseClient } from '@supabase/supabase-js';
import { z } from 'zod';
import type { Environment } from './agent-request';
import { tellByDirectMessage, type Told } from './tell';
import { whoAnswersFor } from './who-answers';

export type LeaveChange = { title: string; body: string };

export type LeaveChangeTold = { messaged: number; failures: string[] };

type LeaveChanger = { environment: Environment; record: SupabaseClient; companyID: string; memberID: string };

const attendancePath = '/attendance/';

const answeredLeaveSchema = z.object({
	result: z.object({
		personID: z.string(),
		person: z.string(),
		kind: z.string(),
		status: z.string(),
		startDate: z.string(),
		endDate: z.string(),
		replacedLeave: z.object({ startDate: z.string(), endDate: z.string() }).optional()
	})
});

type AnsweredLeave = z.infer<typeof answeredLeaveSchema>['result'];

export function leaveChangeOf(toolName: string, body: unknown, memberID: string): LeaveChange | null {
	const answered = answeredLeaveSchema.safeParse(body);
	if (!answered.success || answered.data.result.personID !== memberID) return null;
	const leave = answered.data.result;
	if (toolName === 'leave_delete') return takenBack(leave);
	if (toolName === 'leave_request') return replaced(leave);
	return null;
}

function takenBack(leave: AnsweredLeave): LeaveChange | null {
	if (leave.status !== 'approved') return null;
	return { title: `휴가 취소: ${leave.person}`, body: `${leave.kind} ${daysOf(leave)}` };
}

function replaced(leave: AnsweredLeave): LeaveChange | null {
	if (!leave.replacedLeave) return null;
	const wanted = leave.status === 'approved' ? '휴가 변경' : '휴가 변경 요청';
	return {
		title: `${wanted}: ${leave.person}`,
		body: `${leave.kind} ${daysOf(leave.replacedLeave)} → ${daysOf(leave)}`
	};
}

function daysOf(span: { startDate: string; endDate: string }): string {
	return span.startDate === span.endDate ? span.startDate : `${span.startDate} ~ ${span.endDate}`;
}

export async function withTheLeaveChangeTold(
	changer: LeaveChanger,
	toolName: string,
	body: unknown
): Promise<unknown> {
	const change = leaveChangeOf(toolName, body, changer.memberID);
	if (!change) return body;
	return { ...(body as Record<string, unknown>), notified: await toldWhoAnswers(changer, change) };
}

async function toldWhoAnswers(changer: LeaveChanger, change: LeaveChange): Promise<LeaveChangeTold> {
	try {
		const answerers = await whoAnswersFor(changer.record, changer.companyID, changer.memberID);
		const deliveries = await Promise.all(
			answerers.map((memberID) =>
				tellByDirectMessage(
					changer.environment,
					{ memberID, category: 'leave', ...change, openPath: attendancePath, senderMemberID: changer.memberID },
					changer.record
				)
			)
		);
		return toldOf(deliveries);
	} catch (refusal) {
		return { messaged: 0, failures: [refusal instanceof Error ? refusal.message : String(refusal)] };
	}
}

function toldOf(deliveries: Told[]): LeaveChangeTold {
	return {
		messaged: deliveries.filter((delivery) => delivery.messaged).length,
		failures: deliveries.flatMap((delivery) => (delivery.failure ? [delivery.failure] : []))
	};
}
