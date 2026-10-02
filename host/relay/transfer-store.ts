import { assetBucket, attachmentAddress } from './asset-store';

// Supabase resumable uploads take 6 MiB at a time and no other chunk size
// (https://supabase.com/docs/guides/storage/uploads/resumable-uploads).
export const transferChunkBytes = 6 * 1024 * 1024;

const tusVersion = '1.0.0';
const contentRangePattern = /^bytes (?:\d+-\d+|\*)\/(\d+)$/;

export type RangedSource = (rangeHeader: string) => Promise<Response>;

export type StoreAccess = {
	projectURL: string;
	apiKey: string;
	accessToken: () => Promise<string>;
};

export type CopyProgress = (copiedBytes: number, totalBytes: number) => void;

export type CopiedObject = { path: string; sizeBytes: number; wasAlreadyKept: boolean };

export class TransferFailed extends Error {
	constructor(
		readonly status: number,
		message: string
	) {
		super(message);
		this.name = 'TransferFailed';
	}
}

type Range = { bytes: Uint8Array<ArrayBuffer>; totalBytes: number };

export async function* rangesOf(source: RangedSource): AsyncGenerator<Range> {
	let firstByte = 0;
	for (;;) {
		const response = await source(`bytes=${firstByte}-${firstByte + transferChunkBytes - 1}`);
		if (response.status === 416) return;
		if (response.status === 200) {
			yield await wholeBodyWithinOneRange(response);
			return;
		}
		if (response.status !== 206) throw new TransferFailed(response.status, await refusalOf(response, firstByte));
		const totalBytes = totalBytesOf(response);
		const bytes = new Uint8Array(await response.arrayBuffer());
		yield { bytes, totalBytes };
		firstByte += bytes.byteLength;
		if (bytes.byteLength === 0 || firstByte >= totalBytes) return;
	}
}

async function wholeBodyWithinOneRange(response: Response): Promise<Range> {
	const declared = Number(response.headers.get('content-length') ?? '0');
	if (declared > transferChunkBytes) {
		throw new TransferFailed(502, `the source ignored the range asked of it and offered ${declared} bytes at once`);
	}
	const bytes = new Uint8Array(await response.arrayBuffer());
	return { bytes, totalBytes: bytes.byteLength };
}

function totalBytesOf(response: Response): number {
	const matched = contentRangePattern.exec(response.headers.get('content-range') ?? '');
	if (!matched) throw new TransferFailed(502, 'the source answered a range without saying how large the file is');
	return Number(matched[1]);
}

async function refusalOf(response: Response, firstByte: number): Promise<string> {
	const said = (await response.text().catch(() => '')).trim().slice(0, 300);
	return `the source answered ${response.status} for the bytes from ${firstByte}${said ? `: ${said}` : ''}`;
}

export async function copyIntoStore(
	access: StoreAccess,
	path: string,
	contentType: string,
	source: RangedSource,
	options: { onProgress?: CopyProgress; expectedDigest?: string } = {}
): Promise<CopiedObject> {
	const ranges = rangesOf(source);
	const first = await ranges.next();
	if (first.done) return putEmptyObject(access, path, contentType);
	const totalBytes = first.value.totalBytes;
	const location = await createResumableUpload(access, path, contentType, totalBytes);
	if (!location) return { path, sizeBytes: totalBytes, wasAlreadyKept: true };

	const hasher = new Bun.CryptoHasher('sha256');
	let copiedBytes = 0;
	for await (const bytes of exactChunksOf(first.value.bytes, ranges)) {
		hasher.update(bytes);
		const isLast = copiedBytes + bytes.byteLength >= totalBytes;
		if (isLast && options.expectedDigest) await refuseUnlessDigestMatches(access, location, hasher, options.expectedDigest);
		await sendChunk(access, location, copiedBytes, bytes);
		copiedBytes += bytes.byteLength;
		options.onProgress?.(copiedBytes, totalBytes);
	}
	return { path, sizeBytes: copiedBytes, wasAlreadyKept: false };
}

async function* exactChunksOf(first: Uint8Array<ArrayBuffer>, rest: AsyncGenerator<Range>): AsyncGenerator<Uint8Array<ArrayBuffer>> {
	let pending = first;
	for await (const range of rest) {
		pending = joined(pending, range.bytes);
		while (pending.byteLength >= transferChunkBytes) {
			yield pending.subarray(0, transferChunkBytes);
			pending = pending.slice(transferChunkBytes);
		}
	}
	if (pending.byteLength > 0) yield pending;
}

function joined(first: Uint8Array<ArrayBuffer>, second: Uint8Array<ArrayBuffer>): Uint8Array<ArrayBuffer> {
	if (first.byteLength === 0) return second;
	const both = new Uint8Array(first.byteLength + second.byteLength);
	both.set(first);
	both.set(second, first.byteLength);
	return both;
}

async function refuseUnlessDigestMatches(
	access: StoreAccess,
	location: string,
	hasher: Bun.CryptoHasher,
	expectedDigest: string
): Promise<void> {
	const digest = hasher.digest('hex');
	if (digest === expectedDigest.toLowerCase()) return;
	await fetch(location, { method: 'DELETE', headers: await tusHeaders(access) }).catch(() => undefined);
	throw new TransferFailed(422, `the file read as ${digest}, not the ${expectedDigest} it was asked for by`);
}

async function tusHeaders(access: StoreAccess): Promise<Record<string, string>> {
	return { Authorization: `Bearer ${await access.accessToken()}`, apikey: access.apiKey, 'Tus-Resumable': tusVersion };
}

function uploadMetadata(path: string, contentType: string): string {
	const encoded = (value: string) => Buffer.from(value).toString('base64');
	return [
		`bucketName ${encoded(assetBucket)}`,
		`objectName ${encoded(path)}`,
		`contentType ${encoded(contentType)}`
	].join(',');
}

async function createResumableUpload(
	access: StoreAccess,
	path: string,
	contentType: string,
	totalBytes: number
): Promise<string | null> {
	const response = await fetch(`${access.projectURL.replace(/\/+$/, '')}/storage/v1/upload/resumable`, {
		method: 'POST',
		headers: {
			...(await tusHeaders(access)),
			'Upload-Length': String(totalBytes),
			'Upload-Metadata': uploadMetadata(path, contentType),
			'x-upsert': 'false'
		}
	});
	const location = response.headers.get('location');
	if (response.status === 201 && location) return new URL(location, access.projectURL).toString();
	const said = await response.text();
	if (isAlreadyKeptRefusal(response.status, said)) return null;
	throw new TransferFailed(response.status, `the store would not open an upload for ${path} (${totalBytes} bytes): ${said}`);
}

function isAlreadyKeptRefusal(status: number, said: string): boolean {
	return status === 409 || said.includes('"409"') || said.toLowerCase().includes('already exists');
}



async function sendChunk(access: StoreAccess, location: string, offset: number, bytes: Uint8Array<ArrayBuffer>): Promise<void> {
	const response = await fetch(location, {
		method: 'PATCH',
		headers: {
			...(await tusHeaders(access)),
			'Upload-Offset': String(offset),
			'Content-Type': 'application/offset+octet-stream'
		},
		body: bytes
	});
	if (response.status !== 204) {
		throw new TransferFailed(response.status, `the store refused the bytes from ${offset}: ${await response.text()}`);
	}
}

async function putEmptyObject(access: StoreAccess, path: string, contentType: string): Promise<CopiedObject> {
	const response = await fetch(attachmentAddress(access.projectURL, path), {
		method: 'POST',
		headers: {
			Authorization: `Bearer ${await access.accessToken()}`,
			apikey: access.apiKey,
			'Content-Type': contentType,
			'x-upsert': 'false'
		},
		body: new Uint8Array(0)
	});
	const isAlreadyKept = !response.ok && isAlreadyKeptRefusal(response.status, await response.text());
	if (!response.ok && !isAlreadyKept) throw new TransferFailed(response.status, `the store would not keep ${path}`);
	return { path, sizeBytes: 0, wasAlreadyKept: isAlreadyKept };
}

export function storeRangeReader(access: StoreAccess, path: string): RangedSource {
	return async (rangeHeader) =>
		fetch(`${access.projectURL.replace(/\/+$/, '')}/storage/v1/object/authenticated/${assetBucket}/${path}`, {
			headers: { Authorization: `Bearer ${await access.accessToken()}`, apikey: access.apiKey, Range: rangeHeader }
		});
}

export async function keptSizeOf(access: StoreAccess, path: string): Promise<number | null> {
	const response = await fetch(`${access.projectURL.replace(/\/+$/, '')}/storage/v1/object/info/authenticated/${assetBucket}/${path}`, {
		headers: { Authorization: `Bearer ${await access.accessToken()}`, apikey: access.apiKey }
	});
	if (!response.ok) return null;
	const info = (await response.json().catch(() => null)) as { size?: unknown } | null;
	return typeof info?.size === 'number' ? info.size : null;
}

export async function signedReadURL(access: StoreAccess, path: string, expiresInSeconds: number): Promise<string> {
	const response = await fetch(`${access.projectURL.replace(/\/+$/, '')}/storage/v1/object/sign/${assetBucket}/${path}`, {
		method: 'POST',
		headers: {
			Authorization: `Bearer ${await access.accessToken()}`,
			apikey: access.apiKey,
			'Content-Type': 'application/json'
		},
		body: JSON.stringify({ expiresIn: expiresInSeconds })
	});
	const signed = (await response.json().catch(() => null)) as { signedURL?: unknown } | null;
	if (!response.ok || typeof signed?.signedURL !== 'string') {
		throw new TransferFailed(response.status === 400 ? 404 : response.status, `the store holds nothing at ${path} this member may read`);
	}
	return `${access.projectURL.replace(/\/+$/, '')}/storage/v1${signed.signedURL}`;
}

export async function copyWithinStore(access: StoreAccess, sourcePath: string, destinationPath: string): Promise<void> {
	const response = await fetch(`${access.projectURL.replace(/\/+$/, '')}/storage/v1/object/copy`, {
		method: 'POST',
		headers: {
			Authorization: `Bearer ${await access.accessToken()}`,
			apikey: access.apiKey,
			'Content-Type': 'application/json'
		},
		body: JSON.stringify({ bucketId: assetBucket, sourceKey: sourcePath, destinationKey: destinationPath })
	});
	if (response.ok) return;
	const said = await response.text();
	if (isAlreadyKeptRefusal(response.status, said)) return;
	throw new TransferFailed(response.status, `the store would not copy ${sourcePath} to ${destinationPath}: ${said}`);
}

export async function removeFromStore(access: StoreAccess, paths: string[]): Promise<void> {
	if (paths.length === 0) return;
	const response = await fetch(`${access.projectURL.replace(/\/+$/, '')}/storage/v1/object/${assetBucket}`, {
		method: 'DELETE',
		headers: {
			Authorization: `Bearer ${await access.accessToken()}`,
			apikey: access.apiKey,
			'Content-Type': 'application/json'
		},
		body: JSON.stringify({ prefixes: paths })
	});
	if (!response.ok) throw new TransferFailed(response.status, `the store would not let go of ${paths.length} copies`);
}

export function storeRangesAsStream(source: RangedSource, onProgress?: CopyProgress): ReadableStream<Uint8Array<ArrayBuffer>> {
	const ranges = rangesOf(source);
	let copiedBytes = 0;
	return new ReadableStream<Uint8Array<ArrayBuffer>>({
		async pull(controller) {
			const next = await ranges.next();
			if (next.done) return controller.close();
			copiedBytes += next.value.bytes.byteLength;
			onProgress?.(copiedBytes, next.value.totalBytes);
			controller.enqueue(next.value.bytes);
		}
	});
}
