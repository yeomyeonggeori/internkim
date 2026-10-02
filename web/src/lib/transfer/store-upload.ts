import { TransferFailed, type TransferProgress } from './host-transfer';

// Supabase resumable uploads take 6 MiB at a time and no other chunk size
// (https://supabase.com/docs/guides/storage/uploads/resumable-uploads).
export const uploadChunkBytes = 6 * 1024 * 1024;

const tusVersion = '1.0.0';
const attemptsPerChunk = 3;
const assetBucket = 'asset';

export type StoreSession = {
	projectURL: string;
	publishableKey: string;
	accessToken: () => Promise<string>;
	companyID: string;
	memberID: string;
};

export function uploadObjectPath(session: StoreSession, uploadID: string): string {
	return `${session.companyID}/person/${session.memberID}/upload/${uploadID}`;
}

async function headersOf(session: StoreSession): Promise<Record<string, string>> {
	return { Authorization: `Bearer ${await session.accessToken()}`, apikey: session.publishableKey, 'Tus-Resumable': tusVersion };
}

function metadataOf(object: string, contentType: string): string {
	const encoded = (value: string) => btoa(String.fromCharCode(...new TextEncoder().encode(value)));
	return `bucketName ${encoded(assetBucket)},objectName ${encoded(object)},contentType ${encoded(contentType)}`;
}

async function openUpload(session: StoreSession, object: string, file: Blob, contentType: string): Promise<string> {
	const response = await fetch(`${session.projectURL.replace(/\/+$/, '')}/storage/v1/upload/resumable`, {
		method: 'POST',
		headers: {
			...(await headersOf(session)),
			'Upload-Length': String(file.size),
			'Upload-Metadata': metadataOf(object, contentType),
			'x-upsert': 'false'
		}
	});
	const location = response.headers.get('location');
	if (response.status !== 201 || !location) {
		throw new TransferFailed(response.status, `the store would not take this file: ${(await response.text()).slice(0, 200)}`);
	}
	return new URL(location, session.projectURL).toString();
}

async function offsetTheStoreHas(session: StoreSession, location: string): Promise<number> {
	const response = await fetch(location, { method: 'HEAD', headers: await headersOf(session) });
	const offset = Number(response.headers.get('upload-offset'));
	if (!response.ok || !Number.isFinite(offset)) throw new TransferFailed(response.status, 'the store lost the upload it had started');
	return offset;
}

async function sendChunk(session: StoreSession, location: string, file: Blob, offset: number): Promise<number> {
	const response = await fetch(location, {
		method: 'PATCH',
		headers: { ...(await headersOf(session)), 'Upload-Offset': String(offset), 'Content-Type': 'application/offset+octet-stream' },
		body: file.slice(offset, offset + uploadChunkBytes)
	});
	if (response.status !== 204) {
		const said = (await response.text()).slice(0, 300);
		throw new TransferFailed(response.status, `the store refused the bytes from ${offset}: ${said}`);
	}
	return Number(response.headers.get('upload-offset'));
}

function isWorthRetrying(failure: unknown): boolean {
	return !(failure instanceof TransferFailed) || failure.status >= 500;
}

async function sendChunkRetrying(session: StoreSession, location: string, file: Blob, offset: number): Promise<number> {
	let resumedAt = offset;
	for (let attempt = 1; ; attempt += 1) {
		try {
			return await sendChunk(session, location, file, resumedAt);
		} catch (failure) {
			if (attempt >= attemptsPerChunk || !isWorthRetrying(failure)) throw failure;
			resumedAt = await offsetTheStoreHas(session, location).catch(() => {
				throw failure;
			});
		}
	}
}

export async function uploadToStore(
	session: StoreSession,
	file: Blob,
	contentType: string,
	onProgress: TransferProgress = () => undefined
): Promise<string> {
	const object = uploadObjectPath(session, crypto.randomUUID());
	const location = await openUpload(session, object, file, contentType);
	let offset = 0;
	while (offset < file.size) {
		offset = await sendChunkRetrying(session, location, file, offset);
		onProgress(offset, file.size);
	}
	return object;
}
