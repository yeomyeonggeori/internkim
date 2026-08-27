import { baseCatalogAnswer, liveParameter, toolReachableBy } from './catalog';
import {
	contentTypeOffered,
	filenameOffered,
	filesPath,
	keepFileInTheBucket,
	materialiseCapability,
	mayWriteAFile,
	oversizeRefusal,
	sizeTheHeaderClaims
} from './files';
import {
	PersonalKeyCache,
	RecordRefused,
	callerOfPersonalKey,
	type Caller,
	type CallerPermission,
	type ControlPlaneCredentials
} from './personal-key';

export type CompanyCall = {
	requestID: string;
	capability: string;
	body?: Record<string, unknown>;
};

export type CompanyAnswer = {
	requestID: string;
	status: number;
	body: unknown;
};

export type CompanyCallsBinding = {
	callCompany(companyID: string, call: CompanyCall): Promise<CompanyAnswer>;
};

export type WorkerEnvironment = {
	COMPANY_CALLS: CompanyCallsBinding;
	SUPABASE_URL: string;
	SUPABASE_SECRET_KEY: string;
};

const apiPrefix = '/v1';
const personAPICapability = 'person.api.request';
const catalogHeader = 'X-INTERNKIM-CATALOG';

export default {
	async fetch(request: Request, environment: WorkerEnvironment): Promise<Response> {
		try {
			return await route(request, environment);
		} catch (refusal) {
			if (refusal instanceof RecordRefused) return jsonResponse({ error: refusal.message }, 502);
			throw refusal;
		}
	}
} satisfies ExportedHandler<WorkerEnvironment>;

async function route(request: Request, environment: WorkerEnvironment): Promise<Response> {
	const url = new URL(request.url);
	if (url.pathname !== apiPrefix && !url.pathname.startsWith(`${apiPrefix}/`)) {
		return jsonResponse({ error: 'not found' }, 404);
	}

	const presented = bearerOf(request);
	if (!presented) return jsonResponse({ error: 'this call carried no key' }, 401);

	const caller = await keyCacheFor(environment).callerOf(presented, Date.now());
	if (!caller) return jsonResponse({ error: 'this key belongs to nobody' }, 401);

	const path = url.pathname.slice(apiPrefix.length) || '/';
	if (request.method === 'POST' && path === filesPath) {
		return keepThenMaterialise(request, environment, caller, url);
	}
	if (request.method === 'GET' && !asksForTheLiveSet(url)) {
		const answered = discoveryAnswer(path, caller.permission);
		if (answered) return answered;
	}
	return carryToTheCompany(request, environment, caller, path, url);
}

async function keepThenMaterialise(
	request: Request,
	environment: WorkerEnvironment,
	caller: Caller,
	url: URL
): Promise<Response> {
	if (!mayWriteAFile(caller.permission)) {
		return jsonResponse({ error: 'this key may only read, and putting a file somewhere is a write' }, 403);
	}
	const claimed = sizeTheHeaderClaims(request);
	const claimedRefusal = claimed === null ? null : oversizeRefusal(claimed);
	if (claimedRefusal) return jsonResponse({ error: claimedRefusal }, 413);

	const bytes = new Uint8Array(await request.arrayBuffer());
	if (bytes.byteLength === 0) return jsonResponse({ error: 'this call carried no file' }, 400);
	const refusal = oversizeRefusal(bytes.byteLength);
	if (refusal) return jsonResponse({ error: refusal }, 413);

	const kept = await keepFileInTheBucket(
		credentialsOf(environment),
		caller.companyID,
		bytes,
		contentTypeOffered(request)
	);
	return answerOfCompanyCall(environment, caller.companyID, {
		requestID: crypto.randomUUID(),
		capability: materialiseCapability,
		body: {
			requester: caller.email,
			permission: caller.permission,
			digest: kept.digest,
			contentType: kept.contentType,
			filename: filenameOffered(url)
		}
	});
}

function discoveryAnswer(path: string, permission: CallerPermission): Response | null {
	if (path === '/tools') return catalogResponse(baseCatalogAnswer(permission), 200);
	if (!path.startsWith('/tools/')) return null;
	const name = path.slice('/tools/'.length);
	if (name === '' || name.includes('/')) return null;
	const descriptor = toolReachableBy(name, permission);
	if (!descriptor) return catalogResponse({ error: 'not found' }, 404);
	return catalogResponse(descriptor, 200);
}

async function carryToTheCompany(
	request: Request,
	environment: WorkerEnvironment,
	caller: Caller,
	path: string,
	url: URL
): Promise<Response> {
	const payload = await payloadOf(request);
	if (!payload) return jsonResponse({ error: 'this call carried a body that is not a json object' }, 400);

	return answerOfCompanyCall(environment, caller.companyID, {
		requestID: crypto.randomUUID(),
		capability: personAPICapability,
		body: {
			method: request.method,
			path,
			query: queryTheCompanySees(url),
			permission: caller.permission,
			requester: caller.email,
			payload
		}
	});
}

async function answerOfCompanyCall(
	environment: WorkerEnvironment,
	companyID: string,
	call: CompanyCall
): Promise<Response> {
	const answer = await environment.COMPANY_CALLS.callCompany(companyID, call);
	const status = httpStatusOf(answer.status);
	if (!status) {
		return jsonResponse({ error: `the company answered ${String(answer.status)}, which is not a status` }, 502);
	}
	return jsonResponse(answer.body, status);
}

function httpStatusOf(offered: unknown): number | null {
	if (typeof offered !== 'number' || !Number.isInteger(offered)) return null;
	if (offered < 100 || offered > 599) return null;
	return offered;
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

function credentialsOf(environment: WorkerEnvironment): ControlPlaneCredentials {
	return { projectURL: environment.SUPABASE_URL, serviceRoleKey: environment.SUPABASE_SECRET_KEY };
}

let sharedKeyCache: PersonalKeyCache | undefined;

function keyCacheFor(environment: WorkerEnvironment): PersonalKeyCache {
	sharedKeyCache ??= new PersonalKeyCache((presented) =>
		callerOfPersonalKey(credentialsOf(environment), presented)
	);
	return sharedKeyCache;
}

function bearerOf(request: Request): string | null {
	const offered = request.headers.get('Authorization') ?? '';
	return offered.startsWith('Bearer ') ? offered.slice('Bearer '.length).trim() || null : null;
}

function jsonResponse(document: unknown, status: number): Response {
	return new Response(JSON.stringify(document), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

function catalogResponse(document: unknown, status: number): Response {
	return new Response(JSON.stringify(document), {
		status,
		headers: { 'Content-Type': 'application/json', [catalogHeader]: 'base' }
	});
}
