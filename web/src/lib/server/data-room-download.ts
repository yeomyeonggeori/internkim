import { error, redirect } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import { originalExtensionOf } from '$lib/data-room/storage-path';
import { watermarkedPDF } from './pdf-watermark';

export type GuestDownload = {
	storagePath: string;
	title: string;
	watermark: string[];
	signedLifetimeSeconds: number;
};

export function downloadedAt(now: Date): string {
	return `${now.toISOString().slice(0, 16).replace('T', ' ')} UTC`;
}

export function attachmentHeader(fileName: string): string {
	const asciiName = fileName.replace(/[^\x20-\x7E]|["\\]/g, '_');
	return `attachment; filename="${asciiName}"; filename*=UTF-8''${encodeURIComponent(fileName)}`;
}

export async function guestDownloadResponse(caller: SupabaseClient, download: GuestDownload): Promise<Response> {
	const extension = originalExtensionOf(download.storagePath) ?? 'bin';
	const fileName = `${download.title}.${extension}`;
	if (extension !== 'pdf') return redirectToSignedFile(caller, download, fileName);
	const { data, error: refusal } = await caller.storage.from('asset').download(download.storagePath);
	if (refusal || !data) error(403, 'original downloads are not permitted');
	const stamped = await watermarkedPDF(new Uint8Array(await data.arrayBuffer()), download.watermark).catch(() =>
		error(422, 'this PDF could not be watermarked, so it cannot be downloaded')
	);
	return new Response(new Blob([stamped.slice()], { type: 'application/pdf' }), {
		headers: {
			'Content-Type': 'application/pdf',
			'Content-Disposition': attachmentHeader(fileName),
			'Cache-Control': 'no-store'
		}
	});
}

async function redirectToSignedFile(
	caller: SupabaseClient,
	download: GuestDownload,
	fileName: string
): Promise<never> {
	const signed = await caller.storage
		.from('asset')
		.createSignedUrl(download.storagePath, download.signedLifetimeSeconds, { download: fileName });
	if (signed.error) error(403, 'original downloads are not permitted');
	redirect(302, signed.data.signedUrl);
}
