import { environmentOf, type Environment } from '$lib/server/agent-request';
import { askWhoAnswersFor, type AttendanceAsked } from '$lib/server/ask-who-answers';
import { callingMember, type CallingMember } from '$lib/server/member-request';
import { askTheProject } from '$lib/server/project-function';
import { baseCatalogAnswer, liveParameter, toolReachableBy } from '$lib/server/public-api/catalog';
import { fullPublicAPIPermission } from '$lib/public-api-permission';
import { callCompany } from '$lib/server/public-api/company-call';
import {
	AssetStoreRefused,
	contentTypeOffered,
	filenameOffered,
	keepFileInTheBucket,
	materialiseCapability,
	mayWriteAFile,
	oversizeRefusal,
	sizeTheHeaderClaims,
} from '$lib/server/public-api/files';
import {
	previewToolOverTheRecord,
	recordRunsTheTool,
	runToolOverTheRecord
} from '$lib/server/public-api/record';
import { answererOfTool, destroysSomething, permissionForTool } from '$lib/server/public-api/catalog';
import { refusalOfToolInput, toolInputRecovered } from '$lib/server/public-api/tool-input';
import { reachesPermission } from '$lib/public-api-permission';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

const apiRequestCapability = 'person.api.request';
const filesPath = '/files';
const catalogHeader = 'X-INTERNKIM-CATALOG';

export const fallback: RequestHandler = async ({ request, url, params, platform }) => {
	const environment = environmentOf(platform);
	const member = await callingMember(request, environment);
	const path = `/${params.path}`;

	if (request.method === 'POST' && path === filesPath) {
		return keepThenMaterialise(request, url, environment, member);
	}
	if (request.method === 'GET' && !asksForTheLiveSet(url)) {
		const answered = discoveryAnswer(path, member);
		if (answered) return answered;
	}
	const previewed = previewedToolName(request.method, path);
	if (previewed) return previewHere(request, member, previewed);

	const invoked = invokedToolName(request.method, path);
	if (invoked) {
		const refusal = refusalToCarry(invoked);
		if (refusal) return refusal;
		if (recordRunsTheTool(invoked)) return runHere(request, environment, member, invoked);
	}
	return carryToTheCompany(request, url, environment, member, path, invoked);
};

function refusalToCarry(name: string): Response | null {
	if (answererOfTool(name) !== 'local') return null;
	return json(
		{
			error: `${name} is answered by a runtime beside the agent, which this API has no way to reach`
		},
		{ status: 400 }
	);
}

function descriptorTheTokenReaches(name: string, member: CallingMember) {
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
function inputTheToolWillRead(name: string, input: unknown): Record<string, unknown> {
	if (input !== undefined && (typeof input !== 'object' || input === null || Array.isArray(input))) {
		error(400, 'input is the object the tool reads');
	}
	const read = toolInputRecovered(name, input);
	const refusal = refusalOfToolInput(name, read);
	if (refusal) error(400, refusal);
	return read;
}

function invokedToolName(method: string, path: string): string | null {
	if (method !== 'POST') return null;
	const named = path.match(/^\/tools\/([^/]+)\/invoke$/);
	return named ? named[1] : null;
}

function previewedToolName(method: string, path: string): string | null {
	if (method !== 'POST') return null;
	const named = path.match(/^\/tools\/([^/]+)\/target$/);
	return named ? named[1] : null;
}

async function previewHere(request: Request, member: CallingMember, name: string): Promise<Response> {
	const descriptor = descriptorTheTokenReaches(name, member);
	if (!destroysSomething(descriptor)) {
		error(400, `${name} destroys nothing, so there is nothing to look at before calling it`);
	}
	if (!recordRunsTheTool(name)) {
		error(400, `${name} is answered on the company machine, which is where a preview of it lives`);
	}

	const payload = await payloadOf(request);
	if (!payload) error(400, 'this call carried a body that is not a json object');
	const input = inputTheToolWillRead(name, payload.input);

	const answered = await previewToolOverTheRecord(
		member.caller,
		member.record,
		member.memberID,
		name,
		input,
		new Date()
	);
	return json(answered.body, { status: answered.status });
}

async function runHere(
	request: Request,
	environment: Environment,
	member: CallingMember,
	name: string
): Promise<Response> {
	descriptorTheTokenReaches(name, member);

	const payload = await payloadOf(request);
	if (!payload) error(400, 'this call carried a body that is not a json object');
	const input = inputTheToolWillRead(name, payload.input);

	const answered = await runToolOverTheRecord(
		member.caller,
		member.record,
		member.memberID,
		name,
		input,
		new Date()
	);
	return json(
		await withTheCompanyTold(environment, member, name, input as AttendanceAsked, answered.body),
		{ status: answered.status }
	);
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

function discoveryAnswer(path: string, member: CallingMember): Response | null {
	if (path === '/tools') return catalogResponse(baseCatalogAnswer(member.permission), 200);
	if (!path.startsWith('/tools/')) return null;
	const name = path.slice('/tools/'.length);
	if (name === '' || name.includes('/')) return null;
	const descriptor = toolReachableBy(name, member.permission);
	if (!descriptor) return catalogResponse({ error: 'not found' }, 404);
	return catalogResponse(descriptor, 200);
}

async function carryToTheCompany(
	request: Request,
	url: URL,
	environment: Environment,
	member: CallingMember,
	path: string,
	invoked: string | null
): Promise<Response> {
	const payload = await payloadOf(request);
	if (!payload) error(400, 'this call carried a body that is not a json object');
	const carried = invoked
		? { ...payload, input: inputTheToolWillRead(invoked, payload.input) }
		: payload;

	const answer = await callCompany(environment, member.companyID, apiRequestCapability, {
		method: request.method,
		path,
		query: queryTheCompanySees(url),
		permission: member.permission,
		requester: member.email,
		payload: carried
	});
	return json(answer.body, { status: answer.status });
}

async function keepThenMaterialise(
	request: Request,
	url: URL,
	environment: Environment,
	member: CallingMember
): Promise<Response> {
	if (!mayWriteAFile(member.permission)) {
		error(403, 'this token may only read, and putting a file somewhere is a write');
	}
	const claimed = sizeTheHeaderClaims(request);
	const claimedRefusal = claimed === null ? null : oversizeRefusal(claimed);
	if (claimedRefusal) error(413, claimedRefusal);

	const bytes = new Uint8Array(await request.arrayBuffer());
	if (bytes.byteLength === 0) error(400, 'this call carried no file');
	const refusal = oversizeRefusal(bytes.byteLength);
	if (refusal) error(413, refusal);

	const kept = await keptOrRefused(environment, member.companyID, bytes, contentTypeOffered(request));
	const answer = await callCompany(environment, member.companyID, materialiseCapability, {
		requester: member.email,
		permission: member.permission,
		digest: kept.digest,
		contentType: kept.contentType,
		filename: filenameOffered(url)
	});
	return json(answer.body, { status: answer.status });
}

async function keptOrRefused(
	environment: Environment,
	companyID: string,
	bytes: Uint8Array<ArrayBuffer>,
	contentType: string
) {
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	try {
		return await keepFileInTheBucket({ projectURL, serviceRoleKey }, companyID, bytes, contentType);
	} catch (refusal) {
		if (refusal instanceof AssetStoreRefused) error(502, refusal.message);
		throw refusal;
	}
}

async function payloadOf(request: Request): Promise<Record<string, unknown> | null> {
	if (request.method === 'GET' || request.method === 'HEAD') return {};
	const written = await request.text();
	if (written.trim() === '') return {};
	try {
		const parsed: unknown = JSON.parse(written);
		if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) return null;
		return parsed as Record<string, unknown>;
	} catch {
		return null;
	}
}

function asksForTheLiveSet(url: URL): boolean {
	return url.searchParams.get(liveParameter) === 'true';
}

function queryTheCompanySees(url: URL): string {
	const parameters = new URLSearchParams(url.search);
	parameters.delete(liveParameter);
	return parameters.toString();
}

function catalogResponse(document: unknown, status: number): Response {
	return json(document, { status, headers: { [catalogHeader]: 'base' } });
}
