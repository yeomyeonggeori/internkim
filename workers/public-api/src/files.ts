import { assetBucket, attachmentKind, digestOf, sharedAssetPath } from './asset-address';
import { RecordRefused, type CallerPermission, type ControlPlaneCredentials } from './personal-access-token';

export const filesPath = '/files';
export const materialiseCapability = 'person.api.file';
export const filenameParameter = 'filename';

// Supabase Storage accepts 50 MB for one object by default.
export const largestFileAnAttachmentCanBe = 25 * 1024 * 1024;

const defaultContentType = 'application/octet-stream';
const alreadyKeptStatus = 409;

export type PutDocument = (
	url: string,
	options: { method: string; headers: Record<string, string>; body: Uint8Array<ArrayBuffer> }
) => Promise<{ ok: boolean; status: number }>;

export type KeptFile = {
	path: string;
	digest: string;
	contentType: string;
	sizeBytes: number;
};

const putThroughTheRuntime: PutDocument = (url, options) => fetch(url, options);

export function mayWriteAFile(permission: CallerPermission): boolean {
	return permission !== 'read';
}

export function contentTypeOffered(request: Request): string {
	return request.headers.get('Content-Type')?.trim() || defaultContentType;
}

export function filenameOffered(url: URL): string {
	return url.searchParams.get(filenameParameter)?.trim() ?? '';
}

export function oversizeRefusal(sizeBytes: number): string | null {
	if (sizeBytes <= largestFileAnAttachmentCanBe) return null;
	return `this file is ${sizeBytes} bytes, over the ${largestFileAnAttachmentCanBe} this API keeps`;
}

export function sizeTheHeaderClaims(request: Request): number | null {
	const written = request.headers.get('Content-Length')?.trim();
	if (!written) return null;
	const claimed = Number(written);
	if (!Number.isInteger(claimed) || claimed < 0) return null;
	return claimed;
}

export async function keepFileInTheBucket(
	credentials: ControlPlaneCredentials,
	companyID: string,
	bytes: Uint8Array<ArrayBuffer>,
	contentType: string,
	putDocument: PutDocument = putThroughTheRuntime
): Promise<KeptFile> {
	const digest = await digestOf(bytes);
	const path = sharedAssetPath(companyID, attachmentKind, digest, contentType);
	const response = await putDocument(objectAddress(credentials.projectURL, path), {
		method: 'POST',
		headers: {
			apikey: credentials.serviceRoleKey,
			Authorization: `Bearer ${credentials.serviceRoleKey}`,
			'Content-Type': contentType,
			'x-upsert': 'false'
		},
		body: bytes
	});
	if (!response.ok && response.status !== alreadyKeptStatus) {
		throw new RecordRefused(`the asset store answered ${response.status} for this file`);
	}
	return { path, digest, contentType, sizeBytes: bytes.byteLength };
}

function objectAddress(projectURL: string, path: string): string {
	return `${projectURL.replace(/\/+$/, '')}/storage/v1/object/${assetBucket}/${path}`;
}
