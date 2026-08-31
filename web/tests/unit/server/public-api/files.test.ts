import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import {
	assetBucket,
	attachmentKind,
	extensionOf,
	sharedAssetPath
} from '$lib/server/public-api/asset-address';
import {
	contentTypeOffered,
	filenameOffered,
	keepFileInTheBucket,
	largestFileAnAttachmentCanBe,
	mayWriteAFile,
	oversizeRefusal,
	sizeTheHeaderClaims,
	type PutDocument
} from '$lib/server/public-api/files';

const credentials = { projectURL: 'https://plane.supabase.co', serviceRoleKey: 'service-role' };

function storeAnswering(statuses: number[]): { putDocument: PutDocument; put: string[] } {
	const put: string[] = [];
	const putDocument: PutDocument = async (url) => {
		put.push(url);
		const status = statuses[put.length - 1] ?? 200;
		return { ok: status < 400, status };
	};
	return { putDocument, put };
}

describe('what a key has to carry to put a file anywhere', () => {
	test('is more than reading, because keeping bytes is a write', () => {
		expect(mayWriteAFile('read')).toBe(false);
		expect(mayWriteAFile('write')).toBe(true);
		expect(mayWriteAFile('delete')).toBe(true);
	});
});

describe('the ceiling on one file', () => {
	test('is the 25 MB a workspace attachment may be', () => {
		expect(largestFileAnAttachmentCanBe).toBe(26_214_400);
		expect(oversizeRefusal(largestFileAnAttachmentCanBe)).toBeNull();
		expect(oversizeRefusal(0)).toBeNull();
	});

	test('names itself and the size when it refuses', () => {
		const refusal = oversizeRefusal(largestFileAnAttachmentCanBe + 1);
		expect(refusal).toContain('26214401');
		expect(refusal).toContain('26214400');
	});

	test('is read from the length the caller claims before the body is read at all', () => {
		const oversize = new Request('https://api.intern.kim/v1/files', {
			method: 'POST',
			headers: { 'Content-Length': '99999999' }
		});
		expect(sizeTheHeaderClaims(oversize)).toBe(99_999_999);
		expect(sizeTheHeaderClaims(new Request('https://api.intern.kim/v1/files'))).toBeNull();
	});
});

describe('what the call says about the file itself', () => {
	test('is the content type it was sent with, or nothing named', () => {
		const png = new Request('https://api.intern.kim/v1/files', {
			method: 'POST',
			headers: { 'Content-Type': 'image/png' },
			body: 'x'
		});
		expect(contentTypeOffered(png)).toBe('image/png');
		expect(contentTypeOffered(new Request('https://api.intern.kim/v1/files'))).toBe(
			'application/octet-stream'
		);
	});

	test('is a filename only when the caller offered one', () => {
		expect(filenameOffered(new URL('https://api.intern.kim/v1/files?filename=report.pdf'))).toBe(
			'report.pdf'
		);
		expect(filenameOffered(new URL('https://api.intern.kim/v1/files'))).toBe('');
	});
});

describe('keeping the bytes in the company bucket', () => {
	const bytes = new Uint8Array(new TextEncoder().encode('a file worth sending'));

	test('addresses the object by the digest of its content, under the company', async () => {
		const { putDocument, put } = storeAnswering([200]);

		const kept = await keepFileInTheBucket(credentials, 'company-1', bytes, 'image/png', putDocument);

		expect(kept.path).toBe(sharedAssetPath('company-1', attachmentKind, kept.digest, 'image/png'));
		expect(kept.path.startsWith('company-1/shared/attachment/')).toBe(true);
		expect(kept.path.endsWith('.png')).toBe(true);
		expect(kept.sizeBytes).toBe(bytes.byteLength);
		expect(put[0]).toBe(`${credentials.projectURL}/storage/v1/object/${assetBucket}/${kept.path}`);
	});

	test('lands on one address when the same bytes arrive twice', async () => {
		const { putDocument, put } = storeAnswering([200, 409]);

		const first = await keepFileInTheBucket(credentials, 'company-1', bytes, 'image/png', putDocument);
		const again = await keepFileInTheBucket(
			credentials,
			'company-1',
			new Uint8Array(bytes),
			'image/png',
			putDocument
		);

		expect(again).toEqual(first);
		expect(put[0]).toBe(put[1]);
	});

	test('refuses loudly when the store answers anything else', async () => {
		const { putDocument } = storeAnswering([503]);

		expect(
			keepFileInTheBucket(credentials, 'company-1', bytes, 'image/png', putDocument)
		).rejects.toThrow('503');
	});
});

describe('the addressing this app mirrors from the relay', () => {
	const canonical = readFileSync(
		new URL('../../../../../host/relay/asset-store.ts', import.meta.url),
		'utf8'
	);

	test('names the same bucket and the same kind the relay writes under', () => {
		expect(canonical).toContain(`export const assetBucket = '${assetBucket}';`);
		expect(canonical).toContain(`export const attachmentKind = '${attachmentKind}';`);
	});

	test('builds the same path from the same four parts', () => {
		expect(canonical).toContain(
			'return `${companyID}/shared/${kind}/${digest}${extensionOf(contentType)}`;'
		);
	});

	test('gives the same extension to the same content type', () => {
		for (const [contentType, extension] of Object.entries({
			'image/png': '.png',
			'image/jpeg': '.jpg',
			'image/gif': '.gif',
			'image/webp': '.webp',
			'image/svg+xml': '.svg'
		})) {
			expect(canonical).toContain(`'${contentType}': '${extension}'`);
			expect(extensionOf(contentType)).toBe(extension);
		}
		expect(extensionOf('application/pdf')).toBe('');
	});
});
