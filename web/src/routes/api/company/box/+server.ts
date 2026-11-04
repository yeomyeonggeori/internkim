import { error, json } from '@sveltejs/kit';
import { boxClaimSchema, boxPlacementChangeSchema, boxReleaseSchema, boxServeSchema } from '$lib/company/box';
import { environmentOfPlatform } from '$lib/server/agent-request';
import { BoxRefused, claimBox, companyBoxesOf, emptyBoxesAt, makeBoxServe, placeBox, releaseBox } from '$lib/server/box';
import { callingHostAdministrator } from '$lib/server/company-host-setup';
import type { RequestHandler } from './$types';

const privateHeaders = { 'Cache-Control': 'no-store', 'Pragma': 'no-cache' };

export const GET: RequestHandler = async ({ request, platform, getClientAddress }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	return json(
		{
			...(await companyBoxesOf(member.record, member.companyID)),
			empty: await emptyBoxesAt(member.record, getClientAddress())
		},
		{ headers: privateHeaders }
	);
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	const claim = boxClaimSchema.safeParse(await request.json().catch(() => null));
	if (!claim.success) {
		error(400, 'name the box to connect by its public key, the ticket its code was verified for, and whether it adds to or replaces the company\'s boxes');
	}

	try {
		await claimBox(member.record, member.companyID, claim.data.publicKey, claim.data.ticket, claim.data.intent);
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(refusal.status, refusal.message);
		throw refusal;
	}
	return json(await companyBoxesOf(member.record, member.companyID), { headers: privateHeaders });
};

export const DELETE: RequestHandler = async ({ request, platform }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	const release = boxReleaseSchema.safeParse(await request.json().catch(() => null));
	if (!release.success) error(400, 'name the box to disconnect by its public key');
	if (!(await releaseBox(member.record, member.companyID, release.data.publicKey))) {
		error(404, 'this company has no such box connected');
	}
	return json(await companyBoxesOf(member.record, member.companyID), { headers: privateHeaders });
};

export const PATCH: RequestHandler = async ({ request, platform }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	const serve = boxServeSchema.safeParse(await request.json().catch(() => null));
	if (!serve.success) error(400, 'name the box that serves the company by its public key');
	const outcome = await makeBoxServe(member.record, member.companyID, serve.data.publicKey);
	if (outcome === 'not_its_box') error(404, 'this company has no such box connected');
	if (outcome === 'served_by_connection_file') {
		error(409, 'the company computer connected with a connection file serves the company, and no box can take over what it holds');
	}
	if (outcome === 'no_backup') {
		error(409, 'the company keeps no backup yet for this box to take over from; the box that serves keeps one every fifteen minutes');
	}
	return json(await companyBoxesOf(member.record, member.companyID), { headers: privateHeaders });
};

export const PUT: RequestHandler = async ({ request, platform }) => {
	const member = await callingHostAdministrator(request, environmentOfPlatform(platform?.env));
	const placement = boxPlacementChangeSchema.safeParse(await request.json().catch(() => null));
	if (!placement.success) error(400, 'name the box by its public key with its site');
	const { publicKey, ...where } = placement.data;
	try {
		if (!(await placeBox(member.record, member.companyID, publicKey, where))) error(404, 'this company has no such box connected');
	} catch (refusal) {
		if (refusal instanceof BoxRefused) error(refusal.status, refusal.message);
		throw refusal;
	}
	return json(await companyBoxesOf(member.record, member.companyID), { headers: privateHeaders });
};
