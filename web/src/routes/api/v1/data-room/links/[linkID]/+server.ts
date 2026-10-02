import { error, json, type Cookies } from '@sveltejs/kit';
import { z } from 'zod';
import { environmentOf } from '$lib/server/agent-request';
import {
	dataRoomCookieName,
	dataRoomPlane,
	dataRoomLinkCaller,
	unlockDataRoomLink
} from '$lib/server/data-room-link-request';
import { dataRoomUnlockSchema } from '$lib/data-room/links';
import { sharedDataRoomSchema } from '$lib/data-room/schemas';
import type { RequestHandler } from './$types';

type DataRoomLinkRequest = Pick<
	Parameters<RequestHandler>[0],
	'request' | 'params' | 'platform' | 'url' | 'getClientAddress'
> & {
	cookies: Pick<Cookies, 'get' | 'set'>;
};

export const POST = async ({
	request,
	params,
	platform,
	cookies,
	url,
	getClientAddress
}: DataRoomLinkRequest) => {
	if (!z.string().uuid().safeParse(params.linkID).success) error(404, 'share link not found');
	if (request.headers.get('origin') !== url.origin)
		error(403, 'use the share page to open this link');
	const input = dataRoomUnlockSchema.safeParse(await request.json().catch(() => null));
	if (!input.success) error(400, 'enter six digits and acknowledge the notice');
	const token = await unlockDataRoomLink(
		dataRoomPlane(environmentOf(platform)),
		params.linkID,
		input.data.accessCode,
		input.data.noticeVersion,
		getClientAddress()
	);
	cookies.set(dataRoomCookieName, token.accessToken, {
		httpOnly: true,
		secure: url.protocol === 'https:',
		sameSite: 'strict',
		path: `/api/v1/data-room/links/${params.linkID}`,
		expires: new Date(token.expiresAt * 1000)
	});
	return json({ opened: true }, { headers: { 'Cache-Control': 'no-store' } });
};

export const GET = async ({ params, platform, cookies, url }: DataRoomLinkRequest) => {
	if (!z.string().uuid().safeParse(params.linkID).success) error(404, 'share link not found');
	const { caller, companyID, expiresAt, canDownload } = await dataRoomLinkCaller(
		dataRoomPlane(environmentOf(platform)),
		params.linkID,
		cookies.get(dataRoomCookieName)
	);
	const documentID = url.searchParams.get('documentID');
	if (documentID) {
		if (!z.string().uuid().safeParse(documentID).success) error(404, 'document not found');
		const { data, error: refusal } = await caller
			.from('company_document')
			.select('storage_path')
			.eq('company_id', companyID)
			.eq('id', documentID)
			.not('category_code', 'is', null)
			.maybeSingle();
		if (refusal) error(502, 'the document could not be read');
		if (!data?.storage_path) error(404, 'document not found');
		const lifetime = Math.max(1, Math.min(60, Math.floor((expiresAt - Date.now()) / 1000)));
		const signed = await caller.storage.from('asset').createSignedUrl(data.storage_path, lifetime);
		if (signed.error) error(403, 'original downloads are not permitted');
		return json(
			{ downloadURL: signed.data.signedUrl },
			{ headers: { 'Cache-Control': 'no-store' } }
		);
	}
	const [categories, documents] = await Promise.all([
		caller
			.from('data_room_category')
			.select('code,parent,name,name_ko,description,slug')
			.eq('company_id', companyID)
			.order('position'),
		caller
			.from('company_document')
			.select('id,title,summary,category_code,document_date,status')
			.eq('company_id', companyID)
			.not('category_code', 'is', null)
			.order('issued_at', { ascending: false })
	]);
	if (categories.error || documents.error) error(502, 'the data room could not be read');
	return json(
		{
			...sharedDataRoomSchema.parse({ categories: categories.data, documents: documents.data }),
			canDownload
		},
		{ headers: { 'Cache-Control': 'no-store' } }
	);
};
