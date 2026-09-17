import { error, json } from '@sveltejs/kit';
import { environmentOf, type Environment } from '$lib/server/agent-request';
import { callingMember, type CallingMember } from '$lib/server/member-request';
import {
	claimThePushDevice,
	pushDeviceAddressOf,
	pushDeviceClaimOf,
	pushReachabilityOf,
	releaseThePushDevice
} from '$lib/server/push-device';
import type { RequestHandler } from './$types';

async function memberSignedIntoTheApp(request: Request, environment: Environment): Promise<CallingMember> {
	const member = await callingMember(request, environment);
	if (member.tokenName) error(403, 'push reaches a device the app is signed into, and a personal access token is none');
	return member;
}

async function bodyOf(request: Request): Promise<unknown> {
	return request.json().catch(() => null);
}

export const GET: RequestHandler = async ({ request, platform }) => {
	const member = await memberSignedIntoTheApp(request, environmentOf(platform));
	return json(await pushReachabilityOf(member.caller));
};

export const PUT: RequestHandler = async ({ request, platform }) => {
	const member = await memberSignedIntoTheApp(request, environmentOf(platform));
	return json(await claimThePushDevice(member.caller, pushDeviceClaimOf(await bodyOf(request))));
};

export const DELETE: RequestHandler = async ({ request, url, platform }) => {
	const member = await memberSignedIntoTheApp(request, environmentOf(platform));
	const address = pushDeviceAddressOf(Object.fromEntries(url.searchParams));
	return json(await releaseThePushDevice(member.caller, address));
};
