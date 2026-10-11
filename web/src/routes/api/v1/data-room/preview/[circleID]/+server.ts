import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import type { SupabaseClient } from '@supabase/supabase-js';
import { environmentOf } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { downloadedAt, guestDownloadResponse } from '$lib/server/data-room-download';
import { companyShownToCircle } from '$lib/server/data-room-company';
import { previewedDataRoomSchema, sharedDocumentColumns } from '$lib/data-room/schemas';
import { derivedPath, isDerivedFileName, withOriginalExtension } from '$lib/data-room/storage-path';
import type { RequestHandler } from './$types';

const signedLifetimeSeconds = 60;
const circleIDSchema = z.string().min(1).max(64);
const codesSchema = z.array(z.string()).nullable();

export const GET: RequestHandler = async ({ request, platform, params, url }) => {
	if (!circleIDSchema.safeParse(params.circleID).success) error(404, 'circle not found');
	const member = await callingMember(request, environmentOf(platform));
	const codes = await categoriesTheCircleShows(member.caller, member.companyID, params.circleID);
	const canDownload = url.searchParams.get('canDownload') === '1' && (await circleMayDownload(member.caller, member.companyID, params.circleID));
	const documentID = url.searchParams.get('documentID');
	if (documentID) return previewedFile(member.caller, { companyID: member.companyID, documentID, codes, canDownload, email: member.email, url });
	const [categories, documents, company] = await Promise.all([
		member.caller.from('data_room_category').select('code,parent,name,name_ko,description,slug')
			.eq('company_id', member.companyID).order('position'),
		member.caller.from('company_document').select(sharedDocumentColumns)
			.eq('company_id', member.companyID).in('category_code', codes).order('created_at', { ascending: false }),
		companyShownToCircle(member.caller, member.companyID, params.circleID)
	]);
	if (categories.error || documents.error) error(502, 'the data room could not be read');
	return json(
		previewedDataRoomSchema.parse({
			categories: categories.data,
			documents: documents.data.map(withOriginalExtension),
			company,
			canDownload,
			viewer: null
		}),
		{ headers: { 'Cache-Control': 'no-store' } }
	);
};

async function categoriesTheCircleShows(caller: SupabaseClient, companyID: string, circleID: string): Promise<string[]> {
	const { data, error: refusal } = await caller.rpc('data_room_circle_categories', { target_company: companyID, target_circle: circleID });
	if (refusal) error(502, 'the circle could not be read');
	const codes = codesSchema.parse(data);
	if (!codes) error(403, 'this circle reads more than you may share');
	return codes;
}

async function circleMayDownload(caller: SupabaseClient, companyID: string, circleID: string): Promise<boolean> {
	const { data, error: refusal } = await caller.rpc('data_room_shareable_circles', { target_company: companyID, download: true });
	if (refusal) error(502, 'the circle could not be read');
	return z.array(z.string()).parse(data).includes(circleID);
}

type PreviewedFile = { companyID: string; documentID: string; codes: string[]; canDownload: boolean; email: string; url: URL };

async function previewedFile(caller: SupabaseClient, file: PreviewedFile): Promise<Response> {
	if (!z.string().uuid().safeParse(file.documentID).success) error(404, 'document not found');
	const { data, error: refusal } = await caller.from('company_document').select('storage_path,title')
		.eq('company_id', file.companyID).eq('id', file.documentID).in('category_code', file.codes).maybeSingle();
	if (refusal) error(502, 'the document could not be read');
	if (!data?.storage_path) error(404, 'document not found');
	if (file.url.searchParams.has('download')) {
		if (!file.canDownload) error(403, 'original downloads are not permitted');
		return guestDownloadResponse(caller, {
			storagePath: data.storage_path,
			title: data.title,
			watermark: [`Preview ${file.email}`, downloadedAt(new Date())],
			signedLifetimeSeconds
		});
	}
	const fileName = file.url.searchParams.get('fileName');
	if (fileName !== null && !isDerivedFileName(fileName)) error(400, 'use one file name');
	const path = fileName ? derivedPath(data.storage_path, file.documentID, fileName) : data.storage_path;
	const signed = await caller.storage.from('asset').createSignedUrl(path, signedLifetimeSeconds);
	if (signed.error) error(404, 'file not found');
	return json({ downloadURL: signed.data.signedUrl }, { headers: { 'Cache-Control': 'no-store' } });
}
