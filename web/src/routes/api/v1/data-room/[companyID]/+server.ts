import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import { environmentOf } from '$lib/server/agent-request';
import { dataRoomCaller } from '$lib/server/data-room-request';
import { sharedDataRoomSchema } from '$lib/data-room/schemas';
import { derivedPath } from '$lib/data-room/storage-path';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, platform, params, url }) => {
	if (!z.string().uuid().safeParse(params.companyID).success) error(404, 'data room not found');
	const caller = await dataRoomCaller(request, environmentOf(platform));
	const documentID = url.searchParams.get('documentID');
	if (documentID) {
		const { data, error: refusal } = await caller.from('company_document')
			.select('storage_path').eq('company_id', params.companyID).eq('id', documentID).maybeSingle();
		if (refusal) error(502, refusal.message);
		if (!data?.storage_path) error(404, 'document not found');
		const fileName = url.searchParams.get('fileName');
		if (fileName && (fileName.includes('/') || fileName === '.' || fileName === '..')) error(400, 'use one file name');
		const path = fileName ? derivedPath(data.storage_path, documentID, fileName) : data.storage_path;
		const signed = await caller.storage.from('asset').createSignedUrl(path, 600);
		if (signed.error) error(404, 'file not found or download not permitted');
		return json({ downloadURL: signed.data.signedUrl });
	}
	const [categories, documents] = await Promise.all([
		caller.from('data_room_category').select('code,parent,name,name_ko,description,slug')
			.eq('company_id', params.companyID).order('position'),
		caller.from('company_document').select('id,title,summary,category_code,document_date,status')
			.eq('company_id', params.companyID).not('category_code', 'is', null).order('issued_at', { ascending: false })
	]);
	if (categories.error || documents.error) error(502, 'the data room could not be read');
	return json(sharedDataRoomSchema.parse({ categories: categories.data, documents: documents.data }));
};
