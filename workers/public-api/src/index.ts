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
	PersonalTokenCache,
	RecordRefused,
	callerOfPersonalAccessToken,
	issueToken,
	permissionNamed,
	reachesRung,
	revokeToken,
	tokensOfMember,
	type Caller,
	type CallerPermission,
	type ControlPlaneCredentials
} from './personal-access-token';

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
const tokensPath = '/tokens';
const tokenPath = '/token';
const longestTokenName = 64;
const unnamedTokenPrefix = 'pat-';
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
	if (!presented) return jsonResponse({ error: 'this call carried no token' }, 401);

	const caller = await keyCacheFor(environment).callerOf(presented, Date.now());
	if (!caller) return jsonResponse({ error: 'this token belongs to nobody' }, 401);

	const path = url.pathname.slice(apiPrefix.length) || '/';
	if (path === tokensPath || path === tokenPath) {
		return answerAboutTokens(request, environment, caller, url, path);
	}
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
		return jsonResponse({ error: 'this token may only read, and putting a file somewhere is a write' }, 403);
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

// A token makes and revokes tokens so a caller can rotate one without a browser.
// It may never mint a rung above its own, and it may not revoke itself: the new
// token has to be shown working before the one that made it can be taken away.
async function answerAboutTokens(
	request: Request,
	environment: WorkerEnvironment,
	caller: Caller,
	url: URL,
	path: string
): Promise<Response> {
	const credentials = credentialsOf(environment);
	if (path === tokensPath) {
		if (request.method !== 'GET') {
			return jsonResponse({ error: `one token is made and revoked at ${apiPrefix}${tokenPath}` }, 405);
		}
		return jsonResponse({ tokens: await tokensOfMember(credentials, caller.memberID) }, 200);
	}
	if (request.method === 'POST') {
		const asked = (await request.json().catch(() => null)) as { name?: unknown; permission?: unknown } | null;
		if (!asked || typeof asked !== 'object') {
			return jsonResponse({ error: 'this call carried a body that is not a json object' }, 400);
		}
		const namedByTheCaller = typeof asked.name === 'string' ? asked.name.trim() : '';
		const name = namedByTheCaller || nextUnusedName(await tokensOfMember(credentials, caller.memberID));
		if (name.length > longestTokenName) return jsonResponse({ error: 'that name is too long for a token' }, 400);
		if (name === caller.tokenName) {
			return jsonResponse({ error: 'that name belongs to the token making this call' }, 409);
		}
		const permission = asked.permission === undefined ? caller.permission : permissionNamed(asked.permission);
		if (asked.permission !== undefined && permission !== asked.permission) {
			return jsonResponse({ error: 'a token reads, writes or deletes' }, 400);
		}
		if (!reachesRung(caller.permission, permission)) {
			return jsonResponse({ error: `this token may not make one that ${permission}s` }, 403);
		}
		const token = await issueToken(credentials, caller.memberID, name, permission);
		return jsonResponse({ name, permission, token }, 200);
	}
	if (request.method === 'DELETE') {
		const name = (url.searchParams.get('name') ?? '').trim();
		if (!name) return jsonResponse({ error: 'a revocation names the token' }, 400);
		if (name === caller.tokenName) {
			return jsonResponse({ error: 'a token cannot revoke itself; revoke it with the one that replaced it' }, 409);
		}
		if (!(await revokeToken(credentials, caller.memberID, name))) {
			return jsonResponse({ error: 'no token of yours goes by that' }, 404);
		}
		return jsonResponse({ forgotten: name }, 200);
	}
	return jsonResponse({ error: `the tokens a member holds are listed at ${apiPrefix}${tokensPath}` }, 405);
}

// A caller who does not care what the token is called still needs the names to
// differ, because a name is what a revocation aims at.
function nextUnusedName(held: { name: string }[]): string {
	const taken = new Set(held.map((token) => token.name));
	for (let ordinal = 1; ; ordinal += 1) {
		const name = `${unnamedTokenPrefix}${ordinal}`;
		if (!taken.has(name)) return name;
	}
}

function credentialsOf(environment: WorkerEnvironment): ControlPlaneCredentials {
	return { projectURL: environment.SUPABASE_URL, serviceRoleKey: environment.SUPABASE_SECRET_KEY };
}

let sharedKeyCache: PersonalTokenCache | undefined;

function keyCacheFor(environment: WorkerEnvironment): PersonalTokenCache {
	sharedKeyCache ??= new PersonalTokenCache((presented) =>
		callerOfPersonalAccessToken(credentialsOf(environment), presented)
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
