import { environmentOf, type Environment } from '$lib/server/agent-request';
import { announceHandWrittenAttendance } from '$lib/server/announce-hand-written-attendance';
import { announceClock } from '$lib/server/announce-attendance';
import { vapidKeysInUse } from '$lib/server/vapid-keys';
import { callingMember, type CallingMember } from '$lib/server/member-request';
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
import { recordRunsTheTool, runToolOverTheRecord } from '$lib/server/public-api/record';
import { permissionForTool } from '$lib/server/public-api/catalog';
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
	const invoked = invokedToolName(request.method, path);
	if (invoked && recordRunsTheTool(invoked)) return runHere(request, environment, member, invoked);
	return carryToTheCompany(request, url, environment, member, path);
};

function invokedToolName(method: string, path: string): string | null {
	if (method !== 'POST') return null;
	const named = path.match(/^\/tools\/([^/]+)\/invoke$/);
	return named ? named[1] : null;
}

async function runHere(
	request: Request,
	environment: Environment,
	member: CallingMember,
	name: string
): Promise<Response> {
	const descriptor = toolReachableBy(name, member.permission);
	if (!descriptor) {
		const known = toolReachableBy(name, fullPublicAPIPermission);
		if (!known) error(404, `no tool here goes by ${name}`);
		error(403, `this token may only ${member.permission}, and ${name} ${permissionForTool(known)}s`);
	}
	if (!reachesPermission(member.permission, permissionForTool(descriptor))) {
		error(403, `this token may not ${permissionForTool(descriptor)}`);
	}

	const payload = await payloadOf(request);
	if (!payload) error(400, 'this call carried a body that is not a json object');
	const input = payload.input;
	if (input !== undefined && (typeof input !== 'object' || input === null || Array.isArray(input))) {
		error(400, 'input is the object the tool reads');
	}

	const answered = await runToolOverTheRecord(
		member.caller,
		member.memberID,
		name,
		(input as Record<string, unknown>) ?? {},
		new Date()
	);
	return json(await withTheCompanyTold(environment, member, answered.body), { status: answered.status });
}

type AttendanceWrite = { status: string; eventID: string | null; backdated: boolean };

async function withTheCompanyTold(
	environment: Environment,
	member: CallingMember,
	body: unknown
): Promise<unknown> {
	const written = attendanceWrittenIn(body);
	if (!written) return body;
	try {
		return { ...(body as Record<string, unknown>), notified: await announce(environment, member, written) };
	} catch (refusal) {
		const reason = refusal instanceof Error ? refusal.message : String(refusal);
		return { ...(body as Record<string, unknown>), notified: { failures: [reason] } };
	}
}

async function announce(
	environment: Environment,
	member: CallingMember,
	written: AttendanceWrite
): Promise<unknown> {
	if (written.backdated) {
		if (!written.eventID) return { failures: ['the record did not name the row it wrote'] };
		return announceHandWrittenAttendance(
			environment,
			member.record,
			member.memberID,
			written.eventID,
			written.status
		);
	}
	if (written.status !== 'added') return { told: 0, reached: 0 };
	const keys = await vapidKeysInUse(member.record, environment);
	if (!keys) return { told: 0, reached: 0 };
	return announceClock(member.caller, member.record, member.memberID, keys, Math.floor(Date.now() / 1000));
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
	path: string
): Promise<Response> {
	const payload = await payloadOf(request);
	if (!payload) error(400, 'this call carried a body that is not a json object');

	const answer = await callCompany(environment, member.companyID, apiRequestCapability, {
		method: request.method,
		path,
		query: queryTheCompanySees(url),
		permission: member.permission,
		requester: member.email,
		payload
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
