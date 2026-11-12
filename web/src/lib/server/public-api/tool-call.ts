import { isHttpError } from '@sveltejs/kit';
import { z } from 'zod';
import { askWhoAnswersFor, type AttendanceAsked } from '$lib/server/ask-who-answers';
import { askTheProject } from '$lib/server/project-function';
import { taskChangesAnnouncedAs } from './task-announcement';
import { fullPublicAPIPermission, reachesPermission } from '$lib/public-api-permission';
import { callCompany } from '$lib/server/public-api/company-call';
import { answererOfTool, objectTypesWrittenBy, permissionForTool, toolReachableBy } from '$lib/server/public-api/catalog';
import type { ToolDescriptor } from '$lib/server/public-api/catalog';
import { previewToolOverTheRecord, recordRunsTheTool, runToolOverTheRecord } from '$lib/server/public-api/record';
import type { ToolPlace } from '$lib/server/public-api/record/company';
import { taskLabelGetToolName } from '$lib/server/public-api/catalog/labels';
import { decidedTaskLabelsOf, type TaskLabelDecider } from '$lib/server/public-api/record/task-labels';
import { refusalOfToolInput, toolInputRecovered } from '$lib/server/public-api/tool-input';
import { workspaceFilesCarriedBy } from '$lib/server/public-api/carried-files';
import type { CallingMember } from '$lib/server/member-request';
import type { Environment } from '$lib/server/agent-request';
import { AccessRefused, InputRefused, Refusal, TargetMissing, ToolOutOfReach, answerOfRefusal } from './refusal';

export type ToolCallAnswer = { status: number; body: unknown };

export const apiRequestCapability = 'person.api.request';

const directoryChangedCapability = 'directory.changed';

const objectTypesTheCompanyMirrors = new Set(['circle', 'person']);

export function descriptorTheTokenReaches(name: string, member: Pick<CallingMember, 'permission'>): ToolDescriptor {
	const descriptor = toolReachableBy(name, member.permission);
	if (!descriptor) {
		const known = toolReachableBy(name, fullPublicAPIPermission);
		if (!known) throw new TargetMissing(`no tool here goes by ${name}`);
		throw new AccessRefused(`this token may only ${member.permission}, and ${name} ${permissionForTool(known)}s`);
	}
	if (!reachesPermission(member.permission, permissionForTool(descriptor))) {
		throw new AccessRefused(`this token may not ${permissionForTool(descriptor)}`);
	}
	return descriptor;
}

// The gate answers for the input the tool is then handed, so what it read and
// what runs are the same object rather than two readings of one body.
export function inputTheToolWillRead(name: string, input: unknown): Record<string, unknown> {
	if (input !== undefined && (typeof input !== 'object' || input === null || Array.isArray(input))) {
		throw new InputRefused('input is the object the tool reads');
	}
	const read = toolInputRecovered(name, input);
	const refusal = refusalOfToolInput(name, read);
	if (refusal) throw new InputRefused(refusal);
	return read;
}

function refuseALocalTool(name: string): void {
	if (answererOfTool(name) !== 'local') return;
	throw new ToolOutOfReach(`${name} is answered by a runtime beside the agent, which this API has no way to reach`);
}

export function toolCalledByMember(
	place: ToolPlace,
	member: CallingMember,
	name: string,
	payload: Record<string, unknown>,
	query = ''
): Promise<ToolCallAnswer> {
	return answerOrRefusal(() => toolAnswered(place, member, name, payload, query));
}

export function toolPreviewedByMember(
	member: CallingMember,
	name: string,
	payload: Record<string, unknown>
): Promise<ToolCallAnswer> {
	return answerOrRefusal(() => toolPreviewed(member, name, payload));
}

async function answerOrRefusal(answering: () => Promise<ToolCallAnswer>): Promise<ToolCallAnswer> {
	try {
		return await answering();
	} catch (refusal) {
		if (refusal instanceof Refusal) return answerOfRefusal(refusal);
		if (isHttpError(refusal)) return { status: refusal.status, body: refusal.body };
		throw refusal;
	}
}

function toolPreviewed(member: CallingMember, name: string, payload: Record<string, unknown>): Promise<ToolCallAnswer> {
	descriptorTheTokenReaches(name, member);
	const input = inputTheToolWillRead(name, payload.input);
	return previewToolOverTheRecord(member.caller, member.record, member.memberID, name, input, new Date(),
		workspaceFilesCarriedBy(payload));
}

async function toolAnswered(
	place: ToolPlace,
	member: CallingMember,
	name: string,
	payload: Record<string, unknown>,
	query: string
): Promise<ToolCallAnswer> {
	const { environment } = place;
	refuseALocalTool(name);
	descriptorTheTokenReaches(name, member);
	const input = inputTheToolWillRead(name, payload.input);

	if (!recordRunsTheTool(name)) {
		const answer = await callCompany(environment, member.companyID, apiRequestCapability, {
			method: 'POST',
			path: `/tools/${name}/invoke`,
			query,
			permission: member.permission,
			requester: member.email,
			payload: { ...payload, input }
		});
		return withTheHostRefusalJudged(answer);
	}

	const answered = await runToolOverTheRecord(
		member.caller,
		member.record,
		member.memberID,
		name,
		input,
		new Date(),
		taskLabelsDecidedByTheCompany(environment, member),
		workspaceFilesCarriedBy(payload),
		place,
		taskChangesAnnouncedAs(environment, member.accessToken, place.backgroundWork)
	);
	if (answered.status < 300) await tellTheCompanyWhatItMirrorsChanged(environment, member, name);
	return {
		status: answered.status,
		body: await withTheCompanyTold(environment, member, name, input as AttendanceAsked, answered.body)
	};
}

const hostOutcomeSchema = z.object({ outcome: z.string() });

const statusOfARefusingHostOutcome = new Map([
	['failed', 422],
	['denied', 403]
]);

function withTheHostRefusalJudged(answer: ToolCallAnswer): ToolCallAnswer {
	if (answer.status >= 300) return answer;
	const answered = hostOutcomeSchema.safeParse(answer.body);
	const status = answered.success ? statusOfARefusingHostOutcome.get(answered.data.outcome) : undefined;
	return status ? { status, body: answer.body } : answer;
}

async function tellTheCompanyWhatItMirrorsChanged(environment: Environment, member: CallingMember, name: string) {
	if (!objectTypesWrittenBy(name).some((objectType) => objectTypesTheCompanyMirrors.has(objectType))) return;
	const answer = await callCompany(environment, member.companyID, directoryChangedCapability, {})
		.catch((refusal: unknown) => ({ status: 503, body: refusal }));
	if (answer.status >= 300) console.warn('directory.change_untold', { tool: name, status: answer.status, body: answer.body });
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

function taskLabelsDecidedByTheCompany(environment: Environment, member: CallingMember): TaskLabelDecider {
	return async (draft) => {
		const answer = await callCompany(environment, member.companyID, apiRequestCapability, {
			method: 'POST',
			path: `/tools/${taskLabelGetToolName}/invoke`,
			query: '',
			permission: member.permission,
			requester: member.email,
			payload: { input: draft }
		}).catch((refusal: unknown) => ({ status: 503, body: refusal }));
		const decided = answer.status === 200 ? decidedTaskLabelsOf(answer.body) : null;
		if (!decided) console.warn('task.labels_undecided', { status: answer.status, body: answer.body });
		return decided;
	};
}
