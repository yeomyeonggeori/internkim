import { environmentOf, type Environment } from '$lib/server/agent-request';
import { callingMember, type CallingMember } from '$lib/server/member-request';
import { baseCatalogAnswer, liveParameter, toolReachableBy } from '$lib/server/public-api/catalog';
import { callCompany } from '$lib/server/public-api/company-call';
import {
	companyPictureFormats,
	companyPictureMegabytes,
	companyPicturePath,
	isCompanyPictureFormat,
	refusalOfCompanyPictureSize
} from '$lib/company/company-picture';
import { assetBucket, attachmentKind, companyPictureKind } from '$lib/server/public-api/asset-address';
import { memberPicturePath } from '$lib/profile/member-picture';
import { keepTheMemberPicture } from '$lib/server/public-api/member-picture-keep';
import {
	AssetStoreRefused,
	assetStoreCredentialsOf,
	contentTypeOffered,
	dropFileFromTheBucket,
	filenameOffered,
	keepFileInTheBucket,
	materialiseCapability,
	mayWriteAFile,
	oversizeRefusal,
	sizeTheHeaderClaims,
} from '$lib/server/public-api/files';
import { previewToolOverTheRecord } from '$lib/server/public-api/record';
import {
	apiRequestCapability,
	descriptorTheTokenReaches,
	inputTheToolWillRead,
	toolCalledByMember
} from '$lib/server/public-api/tool-call';
import { error, json } from '@sveltejs/kit';
import { clockAttendance } from '$lib/server/public-api/attendance-clock';
import type { RequestHandler } from './$types';

const filesPath = '/files';
const readableForOneHour = 60 * 60;
const catalogHeader = 'X-INTERNKIM-CATALOG';

export const fallback: RequestHandler = async ({ request, url, params, platform }) => {
	const environment = environmentOf(platform);
	const path = `/${params.path}`;
	if (request.method === 'POST' && path === '/tools/attendance_add/invoke') {
		const clocked = await clockAttendance(request, environment, platform?.context?.waitUntil.bind(platform.context));
		if (clocked) return clocked;
	}
	const member = await callingMember(request, environment);

	if (request.method === 'POST' && path === filesPath) {
		return keepThenMaterialise(request, url, environment, member);
	}
	if (path === companyPicturePath && request.method === 'POST') {
		return keepTheCompanyPicture(request, environment, member);
	}
	if (path === companyPicturePath && request.method === 'DELETE') {
		return forgetTheCompanyPicture(member);
	}
	if (path === memberPicturePath && request.method === 'POST') {
		return keepTheMemberPicture(request, environment, member);
	}
	if (request.method === 'GET' && !asksForTheLiveSet(url)) {
		const answered = discoveryAnswer(path, member);
		if (answered) return answered;
	}
	const previewed = previewedToolName(request.method, path);
	if (previewed) return previewHere(request, member, previewed);

	const invoked = invokedToolName(request.method, path);
	if (invoked) return runHere(request, url, environment, member, invoked);
	return carryToTheCompany(request, url, environment, member, path);
};

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
	descriptorTheTokenReaches(name, member);

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
	url: URL,
	environment: Environment,
	member: CallingMember,
	name: string
): Promise<Response> {
	const payload = await payloadOf(request);
	if (!payload) error(400, 'this call carried a body that is not a json object');

	const answered = await toolCalledByMember(environment, member, name, payload, queryTheCompanySees(url));
	return json(answered.body, { status: answered.status });
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

	const kept = await keptOrRefused(
		environment,
		member.companyID,
		attachmentKind,
		bytes,
		contentTypeOffered(request)
	);
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
	kind: string,
	bytes: Uint8Array<ArrayBuffer>,
	contentType: string
) {
	try {
		return await keepFileInTheBucket(assetStoreCredentialsOf(environment), companyID, kind, bytes, contentType);
	} catch (refusal) {
		if (refusal instanceof AssetStoreRefused) error(502, refusal.message);
		throw refusal;
	}
}

async function keepTheCompanyPicture(
	request: Request,
	environment: Environment,
	member: CallingMember
): Promise<Response> {
	if (!mayWriteAFile(member.permission)) {
		error(403, 'this token may only read, and a company picture is a write');
	}
	const picture = await pictureOffered(request);
	const bytes = new Uint8Array(await picture.arrayBuffer());
	if (bytes.byteLength === 0) error(400, 'this call carried no picture');
	const tooBig = refusalOfCompanyPictureSize(bytes.byteLength);
	if (tooBig) error(413, `this picture is ${tooBig}, over the ${companyPictureMegabytes}MB a company picture may be`);
	const contentType = picture.type.trim();
	if (!isCompanyPictureFormat(contentType)) {
		error(400, `a company picture is one of ${companyPictureFormats.join(', ')}`);
	}

	const kept = await keptOrRefused(environment, member.companyID, companyPictureKind, bytes, contentType);
	try {
		return json({ profileImageURL: await companyPictureWritten(member, kept.path) }, { status: 200 });
	} catch (refusal) {
		await dropFileFromTheBucket(assetStoreCredentialsOf(environment), kept.path).catch(() => undefined);
		throw refusal;
	}
}

async function forgetTheCompanyPicture(member: CallingMember): Promise<Response> {
	if (member.permission === 'read') {
		error(403, 'this token may only read, and taking the company picture down is a write');
	}
	return json({ profileImageURL: await companyPictureWritten(member, null) }, { status: 200 });
}

async function pictureOffered(request: Request): Promise<File> {
	const form = await request.formData().catch(() => null);
	if (!form) error(400, 'a company picture arrives as multipart/form-data in a file field');
	const offered = form.get('file');
	if (!(offered instanceof File)) {
		error(400, 'a company picture arrives as multipart/form-data in a file field');
	}
	return offered;
}

async function companyPictureWritten(
	member: CallingMember,
	path: string | null
): Promise<string | null> {
	const company = await member.caller.from('company').select('id').limit(1).single<{ id: string }>();
	if (company.error) error(500, company.error.message);

	const written = await member.caller
		.from('company')
		.update({ profile_image: path })
		.eq('id', company.data.id)
		.select('id');
	if (written.error) error(422, written.error.message);
	if ((written.data ?? []).length === 0) {
		error(403, 'only an administrator can change the company picture');
	}
	if (!path) return null;

	const signed = await member.caller.storage
		.from(assetBucket)
		.createSignedUrl(path, readableForOneHour);
	if (signed.error) error(502, signed.error.message);
	return signed.data?.signedUrl ?? null;
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
