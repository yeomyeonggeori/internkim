import { error, json } from '@sveltejs/kit';
import { asMember, planeCredentialsOf } from '../control-plane';
import { memberAccessTokenOf } from '../member-request';
import { askTheProject } from '../project-function';
import type { Environment } from '../agent-request';
import { attendanceAddInputSchema } from './catalog/tools';
import { descriptorTheTokenReaches } from './tool-call';
import { toolInputRecovered } from './tool-input';
import { sentencesOfSchemaRefusal } from './schema-sentences';
import { addAttendanceFor } from './record/attendance-tools';
import { RecordRefusedTheWrite } from './record/tasks';

type BackgroundWork = (work: Promise<unknown>) => void;

export async function clockAttendance(
	request: Request,
	environment: Environment,
	backgroundWork?: BackgroundWork
): Promise<Response | null> {
	const payload: unknown = await request.clone().json().catch(() => null);
	if (!payload || typeof payload !== 'object' || !('input' in payload)) return null;

	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the control plane is not configured');
	const member = await memberAccessTokenOf(request, plane);

	const recovered = toolInputRecovered('attendance_add', payload.input);
	const parsed = attendanceAddInputSchema.safeParse(recovered);
	if (!parsed.success) error(400, sentencesOfSchemaRefusal(parsed.error.issues, recovered, 'input'));
	const input = parsed.data;
	if (input.personHint || input.date || input.time) return null;

	descriptorTheTokenReaches('attendance_add', member);
	const caller = asMember(plane, member.accessToken);
	try {
		const result = await addAttendanceFor(caller, null, input);
		const announcement = announceAttendance(environment, member.accessToken, result.status === 'added');
		backgroundWork?.(announcement);
		return json({ tool: 'attendance_add', result });
	} catch (refusal) {
		if (!(refusal instanceof RecordRefusedTheWrite)) throw refusal;
		return json({ error: refusal.message, errorCode: refusal.errorCode }, { status: refusal.status });
	}
}

async function announceAttendance(
	environment: Environment,
	accessToken: string,
	colleagues: boolean
): Promise<void> {
	try {
		const answer = await askTheProject(environment, 'announce-attendance', { what: 'clock', colleagues }, accessToken);
		if (answer.status >= 300) console.error('attendance.announcement_failed', answer.status);
	} catch (failure) {
		console.error('attendance.announcement_failed', failure);
	}
}
