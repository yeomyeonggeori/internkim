import { assetBucket } from './asset-store';
import { TransferFailed, type StoreAccess } from './transfer-store';
import type { MessengerStore } from './messenger-calls';

export function messengerStoreOf(companyID: string, access: StoreAccess): MessengerStore {
	return { companyID, access, signedUploadURL: (path) => signedUploadURL(access, path) };
}

async function signedUploadURL(access: StoreAccess, path: string): Promise<string> {
	const storage = `${access.projectURL.replace(/\/+$/, '')}/storage/v1`;
	const response = await fetch(`${storage}/object/upload/sign/${assetBucket}/${path}`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${await access.accessToken()}`, apikey: access.apiKey }
	});
	const signed = (await response.json().catch(() => null)) as { url?: unknown } | null;
	if (!response.ok || typeof signed?.url !== 'string') {
		throw new TransferFailed(response.status, `the store would not sign an upload to ${path}`);
	}
	return `${storage}${signed.url}`;
}
