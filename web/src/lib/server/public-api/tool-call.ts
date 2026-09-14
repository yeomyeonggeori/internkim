import { error, isHttpError } from '@sveltejs/kit';
import { askWhoAnswersFor, type AttendanceAsked } from '$lib/server/ask-who-answers';
import { askTheProject } from '$lib/server/project-function';
import { fullPublicAPIPermission, reachesPermission } from '$lib/public-api-permission';
import { callCompany } from '$lib/server/public-api/company-call';
import { answererOfTool, permissionForTool, toolReachableBy } from '$lib/server/public-api/catalog';
import type { ToolDescriptor } from '$lib/server/public-api/catalog';
import {
	answerRemembered,
	idempotencyKeyOffered,
	keyRemembersTheWrite,
	rememberTheWrite,
	writeRememberedFor
} from '$lib/server/public-api/idempotency';
import { recordRunsTheTool, runToolOverTheRecord } from '$lib/server/public-api/record';
import { refusalOfToolInput, toolInputRecovered } from '$lib/server/public-api/tool-input';
import type { CallingMember } from '$lib/server/member-request';
import type { Environment } from '$lib/server/agent-request';

export type ToolCallAnswer = { status: number; body: unknown };

export const apiRequestCapability = 'person.api.request';

export function descriptorTheTokenReaches(name: string, member: CallingMember): ToolDescriptor {
	const descriptor = toolReachableBy(name, member.permission);
	if (!descriptor) {
		const known = toolReachableBy(name, fullPublicAPIPermission);
		if (!known) error(404, `no tool here goes by ${name}`);
		error(403, `this token may only ${member.permission}, and ${name} ${permissionForTool(known)}s`);
	}
	if (!reachesPermission(member.permission, permissionForTool(descriptor))) {
		error(403, `this token may not ${permissionForTool(descriptor)}`);
	}
	return descriptor;
}

// The gate answers for the input the tool is then handed, so what it read and
// what runs are the same object rather than two readings of one body.
export function inputTheToolWillRead(name: string, input: unknown): Record<string, unknown> {
	if (input !== undefined && (typeof input !== 'object' || input === null || Array.isArray(input))) {
		error(400, 'input is the object the tool reads');
	}
	const read = toolInputRecovered(name, input);
	const refusal = refusalOfToolInput(name, read);
	if (refusal) error(400, refusal);
	return read;
}

export function refusalOfALocalTool(name: string): ToolCallAnswer | null {
	if (answererOfTool(name) !== 'local') return null;
	return {
		status: 400,
		body: { error: `${name} is answered by a runtime beside the agent, which this API has no way to reach` }
	};
}

export async function toolCalledByMember(
	environment: Environment,
	member: CallingMember,
	name: string,
	payload: Record<string, unknown>,
	query = ''
): Promise<ToolCallAnswer> {
	const local = refusalOfALocalTool(name);
	if (local) return local;

	const descriptor = descriptorTheTokenReaches(name, member);
	const input = inputTheToolWillRead(name, payload.input);

	if (!recordRunsTheTool(name)) {
		return callCompany(environment, member.companyID, apiRequestCapability, {
			method: 'POST',
			path: `/tools/${name}/invoke`,
			query,
			permission: member.permission,
			requester: member.email,
			payload: { ...payload, input }
		});
	}

	const key = idempotencyKeyOffered(payload);
	if (!keyRemembersTheWrite(descriptor, key)) return runOverTheRecord(environment, member, name, input);

	const remembered = await writeRememberedFor(member.caller, member.memberID, key);
	if (remembered) return answerRemembered(remembered, name);
	const answered = await runOverTheRecord(environment, member, name, input);
	await rememberTheWrite(member.caller, member.memberID, key, name, answered);
	return answered;
}

async function runOverTheRecord(
	environment: Environment,
	member: CallingMember,
	name: string,
	input: Record<string, unknown>
): Promise<ToolCallAnswer> {
	const answered = await runToolOverTheRecord(
		member.caller,
		member.record,
		member.memberID,
		name,
		input,
		new Date()
	);
	return {
		status: answered.status,
		body: await withTheCompanyTold(environment, member, name, input as AttendanceAsked, answered.body)
	};
}

export async function toolAnswerOrRefusal(
	environment: Environment,
	member: CallingMember,
	name: string,
	payload: Record<string, unknown>
): Promise<ToolCallAnswer> {
	try {
		return await toolCalledByMember(environment, member, name, payload);
	} catch (refusal) {
		if (isHttpError(refusal)) return { status: refusal.status, body: refusal.body };
		throw refusal;
	}
}

type AttendanceWrite = { status: string; eventID: string | null; backdated: boolean };

async function withTheCompanyTold(
	environment: Environment,
	member: CallingMember,
	name: string,
	asked: AttendanceAsked,
	body: unknown
): Promise<unknown> {
	const written = attendanceWrittenIn(body);
	if (!written) return body;
	try {
		return {
			...(body as Record<string, unknown>),
			notified: await announce(environment, member, name, asked, written)
		};
	} catch (refusal) {
		const reason = refusal instanceof Error ? refusal.message : String(refusal);
		return { ...(body as Record<string, unknown>), notified: { failures: [reason] } };
	}
}

async function announce(
	environment: Environment,
	member: CallingMember,
	name: string,
	asked: AttendanceAsked,
	written: AttendanceWrite
): Promise<unknown> {
	if (written.status === 'asked') {
		return askWhoAnswersFor(environment, member.record, member.memberID, name, asked);
	}
	if (written.backdated || written.status !== 'added') return { told: 0, reached: 0 };
	const answer = await askTheProject(environment, 'announce-attendance', { what: 'clock' }, member.accessToken);
	if (answer.status >= 300) return { told: 0, reached: 0 };
	return answer.body;
}

function attendanceWrittenIn(body: unknown): AttendanceWrite | null {
	const result = (body as { result?: Partial<AttendanceWrite> } | null)?.result;
	if (!result || typeof result.status !== 'string' || typeof result.backdated !== 'boolean') return null;
	return {
		status: result.status,
		eventID: typeof result.eventID === 'string' ? result.eventID : null,
		backdated: result.backdated
	};
}
