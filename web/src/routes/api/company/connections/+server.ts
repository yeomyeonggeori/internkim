import { type PlaneCredentials, adminCallerOf, asMember, controlPlane, planeCredentialsOf } from '$lib/server/control-plane';
import { companyConnectionKinds } from '$lib/company/connections';
import {
	companyConnections,
	forgetCompanyConnection,
	saveCompanyConnection
} from '$lib/server/company-credential';
import { env } from '$env/dynamic/private';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { memberAccessTokenOf } from '$lib/server/member-request';



function planeOf(platform: App.Platform | undefined): PlaneCredentials {
	const plane = planeCredentialsOf({ ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) });
	if (!plane) error(500, 'the central plane is not configured');
	return plane;
}

async function adminOf(request: Request, plane: PlaneCredentials) {
	const { accessToken } = await memberAccessTokenOf(request, plane);
	const caller = await adminCallerOf(asMember(plane, accessToken));
	if (!caller) error(403, 'only an admin keeps these');
	return caller;
}

export const GET: RequestHandler = async ({ request, platform }) => {
	const plane = planeOf(platform);
	const caller = await adminOf(request, plane);
	const client = controlPlane(plane);
	return json({ connections: await companyConnections(client, caller.companyID) });
};

export const PUT: RequestHandler = async ({ request, platform }) => {
	const plane = planeOf(platform);
	const caller = await adminOf(request, plane);

	const body = (await request.json().catch(() => ({}))) as Record<string, unknown>;
	const kind = typeof body.kind === 'string' ? body.kind.trim() : '';
	const host = typeof body.host === 'string' ? body.host.trim() : '';
	if (!companyConnectionKinds.some((known) => known === kind)) error(400, 'unknown kind of connection');
	if (!host) error(400, 'a host is required');

	await saveCompanyConnection(controlPlane(plane), caller.companyID, {
		kind,
		host,
		settings: isRecord(body.settings) ? body.settings : {}
	});
	return json({ kind, host });
};

export const DELETE: RequestHandler = async ({ request, platform, url }) => {
	const plane = planeOf(platform);
	const caller = await adminOf(request, plane);
	const kind = url.searchParams.get('kind') ?? '';
	if (!companyConnectionKinds.some((known) => known === kind)) error(400, 'unknown kind of connection');
	await forgetCompanyConnection(controlPlane(plane), caller.companyID, kind);
	return json({ kind });
};

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}
