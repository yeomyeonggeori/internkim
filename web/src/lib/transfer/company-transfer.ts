import { callCompanyApp, onCompanyEvent } from '$lib/host-bridge';
import { assetBucket, keptAssetPathOf } from '$lib/messenger/kept-attachment';
import { projectURL, publishableKey, supabase } from '$lib/supabase';
import { supabaseMember } from '$lib/supabase-session';
import { TransferFailed, transferThroughTheHost, type HostConnection, type TransferProgress } from './host-transfer';
import { uploadToStore, type StoreSession } from './store-upload';

export { TransferFailed, type TransferProgress };

export type ReadableCopy = { address: string; sizeBytes: number };

const readableForSeconds = 60 * 60;

const hostConnection: HostConnection = { call: callCompanyApp, listen: onCompanyEvent };

async function accessToken(): Promise<string> {
	const { data } = await supabase().auth.getSession();
	const token = data.session?.access_token;
	if (!token) throw new TransferFailed(401, 'sign in first');
	return token;
}

async function storeSession(): Promise<StoreSession> {
	const { companyID, memberID } = await supabaseMember();
	if (!companyID || !memberID) throw new TransferFailed(401, 'sign in first');
	return { projectURL: projectURL(), publishableKey: publishableKey(), accessToken, companyID, memberID };
}

export function readableCopyOf(result: unknown): ReadableCopy {
	const copy = result as { address?: unknown; sizeBytes?: unknown } | null;
	if (typeof copy?.address !== 'string' || copy.address === '') {
		throw new TransferFailed(502, 'the company computer copied the file and said nothing about where');
	}
	return { address: copy.address, sizeBytes: typeof copy.sizeBytes === 'number' ? copy.sizeBytes : 0 };
}

export async function copiedForReading(
	capability: string,
	body: Record<string, unknown>,
	onProgress?: TransferProgress
): Promise<ReadableCopy> {
	return readableCopyOf(await transferThroughTheHost(hostConnection, { capability, body }, onProgress));
}

export async function uploadedThroughTheHost(
	capability: string,
	file: Blob,
	contentType: string,
	body: Record<string, unknown>,
	onProgress: TransferProgress = () => undefined
): Promise<unknown> {
	const object = await uploadToStore(await storeSession(), file, contentType, (sentBytes, totalBytes) =>
		onProgress(sentBytes, totalBytes)
	);
	return transferThroughTheHost(hostConnection, { capability, body: { ...body, object, contentType } });
}

export async function signedForReading(address: string, downloadAs?: string): Promise<string> {
	const path = keptAssetPathOf(projectURL(), address);
	if (!path) throw new TransferFailed(400, 'that copy is not in the company store');
	const signed = await supabase()
		.storage.from(assetBucket)
		.createSignedUrl(path, readableForSeconds, downloadAs ? { download: downloadAs } : undefined);
	if (signed.error || !signed.data?.signedUrl) {
		throw new TransferFailed(403, signed.error?.message ?? 'the store would not sign for this copy');
	}
	return signed.data.signedUrl;
}
