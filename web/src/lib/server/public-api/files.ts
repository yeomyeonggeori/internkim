import { assetBucket, digestOf, sharedAssetPath } from './asset-address';
import type { PublicAPIPermission } from '$lib/public-api-permission';
import type { ControlPlaneCredentials } from '$lib/server/control-plane';

export const materialiseCapability = 'person.api.file';
export const filenameParameter = 'filename';

// Supabase Storage accepts 50 MB for one object by default.
export const largestFileAnAttachmentCanBe = 25 * 1024 * 1024;

const defaultContentType = 'application/octet-stream';
const alreadyKeptStatus = 409;

export type StoreAnswer = { ok: boolean; status: number; text: () => Promise<string> };

export type PutDocument = (
	url: string,
	options: { method: string; headers: Record<string, string>; body?: Uint8Array<ArrayBuffer> }
) => Promise<StoreAnswer>;

export type KeptFile = {
	path: string;
	digest: string;
	contentType: string;
	sizeBytes: number;
	wasAlreadyKept: boolean;
};

const putThroughTheRuntime: PutDocument = (url, options) => fetch(url, options);

export function assetStoreCredentialsOf(environment: Record<string, string | undefined>): ControlPlaneCredentials {
	return {
		projectURL: environment.SUPABASE_URL ?? '',
		serviceRoleKey: environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? ''
	};
}

export function mayWriteAFile(permission: PublicAPIPermission): boolean {
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

export class AssetStoreRefused extends Error {
	constructor(reason: string) {
		super(reason);
		this.name = 'AssetStoreRefused';
	}
}

export async function keepFileInTheBucket(
	credentials: ControlPlaneCredentials,
	companyID: string,
	kind: string,
	bytes: Uint8Array<ArrayBuffer>,
	contentType: string,
	putDocument: PutDocument = putThroughTheRuntime
): Promise<KeptFile> {
	const digest = await digestOf(bytes);
	const path = sharedAssetPath(companyID, kind, digest, contentType);
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
	const wasAlreadyKept = !response.ok;
	if (wasAlreadyKept && !(await namesAFileAlreadyKept(response))) {
		throw new AssetStoreRefused(`the asset store answered ${response.status} for this file`);
	}
	return { path, digest, contentType, sizeBytes: bytes.byteLength, wasAlreadyKept };
}

// The store answers a path it already holds with HTTP 400 and its own
// statusCode of 409, so the HTTP status alone does not say whether the bytes
// are there. Storage API: https://supabase.com/docs/reference/javascript/storage-from-upload
async function namesAFileAlreadyKept(response: StoreAnswer): Promise<boolean> {
	if (response.status === alreadyKeptStatus) return true;
	const written = await response.text().catch(() => '');
	if (!written.trim().startsWith('{')) return false;
	try {
		const said: unknown = JSON.parse(written);
		if (typeof said !== 'object' || said === null) return false;
		return (said as { statusCode?: unknown }).statusCode === String(alreadyKeptStatus);
	} catch {
		return false;
	}
}

export async function dropFileFromTheBucket(
	credentials: ControlPlaneCredentials,
	path: string,
	putDocument: PutDocument = putThroughTheRuntime
): Promise<void> {
	const response = await putDocument(objectAddress(credentials.projectURL, path), {
		method: 'DELETE',
		headers: {
			apikey: credentials.serviceRoleKey,
			Authorization: `Bearer ${credentials.serviceRoleKey}`
		}
	});
	if (!response.ok) {
		throw new AssetStoreRefused(`the asset store answered ${response.status} taking this file back out`);
	}
}

function objectAddress(projectURL: string, path: string): string {
	return `${projectURL.replace(/\/+$/, '')}/storage/v1/object/${assetBucket}/${path}`;
}
