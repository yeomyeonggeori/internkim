import { describe, expect, test } from 'bun:test';
import { handleReleaseRegistryRequest } from '../../../../workers/release-registry/src/registry';

describe('Release Registry Worker', () => {
	test('rejects requests without release download token before reading R2', async () => {
		const bucket = new MemoryR2Bucket({ 'channels/stable.json': 'ok' });

		const response = await handleReleaseRegistryRequest(new Request('https://updates.example.test/channels/stable.json'), {
			RELEASE_BUCKET: bucket as unknown as R2Bucket,
			RELEASE_DOWNLOAD_TOKEN: 'download-token'
		});

		expect(response.status).toBe(401);
		expect(bucket.getCalls).toEqual([]);
	});

	test('serves R2 object when release download token matches', async () => {
		const bucket = new MemoryR2Bucket({ 'channels/stable.json': '{"releaseID":"release-1"}' });

		const response = await handleReleaseRegistryRequest(
			new Request('https://updates.example.test/channels/stable.json', {
				headers: { 'X-INTERNKIM-RELEASE-TOKEN': 'download-token' }
			}),
			{
				RELEASE_BUCKET: bucket as unknown as R2Bucket,
				RELEASE_DOWNLOAD_TOKEN: 'download-token'
			}
		);

		expect(response.status).toBe(200);
		expect(await response.text()).toBe('{"releaseID":"release-1"}');
		expect(bucket.getCalls).toEqual(['channels/stable.json']);
	});

	test('serves the apt repository without a token, because apt carries none', async () => {
		const bucket = new MemoryR2Bucket({
			'deb/dists/stable/InRelease': 'Origin: InternKim',
			'deb/pool/main/i/internkim/internkim_1.0.0_arm64.deb': 'package'
		});

		for (const objectKey of ['deb/dists/stable/InRelease', 'deb/pool/main/i/internkim/internkim_1.0.0_arm64.deb']) {
			const response = await handleReleaseRegistryRequest(new Request(`https://updates.example.test/${objectKey}`), {
				RELEASE_BUCKET: bucket as unknown as R2Bucket,
				RELEASE_DOWNLOAD_TOKEN: 'download-token'
			});
			expect(response.status).toBe(200);
		}
	});

	test('keeps the rest of the bucket behind the token', async () => {
		const bucket = new MemoryR2Bucket({ 'debug/secret': 'secret', 'deb': 'secret' });

		for (const objectKey of ['debug/secret', 'deb']) {
			const response = await handleReleaseRegistryRequest(new Request(`https://updates.example.test/${objectKey}`), {
				RELEASE_BUCKET: bucket as unknown as R2Bucket,
				RELEASE_DOWNLOAD_TOKEN: 'download-token'
			});
			expect(response.status).toBe(401);
		}
		expect(bucket.getCalls).toEqual([]);
	});

	test('rejects unsafe object keys', async () => {
		const bucket = new MemoryR2Bucket({ '../secret': 'secret' });

		const response = await handleReleaseRegistryRequest(
			new Request('https://updates.example.test/%252E%252E/secret', {
				headers: { 'X-INTERNKIM-RELEASE-TOKEN': 'download-token' }
			}),
			{
				RELEASE_BUCKET: bucket as unknown as R2Bucket,
				RELEASE_DOWNLOAD_TOKEN: 'download-token'
			}
		);

		expect(response.status).toBe(404);
		expect(bucket.getCalls).toEqual([]);
	});
});

class MemoryR2Bucket {
	getCalls: string[] = [];

	constructor(private readonly objects: Record<string, string>) {}

	async head(key: string): Promise<R2Object | null> {
		const value = this.objects[key];
		if (value === undefined) return null;
		return createMemoryR2Object(key, value);
	}

	async get(key: string, _options?: R2GetOptions): Promise<R2ObjectBody | null> {
		this.getCalls.push(key);
		const value = this.objects[key];
		if (value === undefined) {
			return null;
		}
		return createMemoryR2ObjectBody(key, value);
	}

	async put(
		_key: string,
		_value: ReadableStream | ArrayBuffer | ArrayBufferView | string | null | Blob,
		_options?: R2PutOptions
	): Promise<R2Object> {
		throw new Error('MemoryR2Bucket.put is not implemented');
	}

	async createMultipartUpload(_key: string, _options?: R2MultipartOptions): Promise<R2MultipartUpload> {
		throw new Error('MemoryR2Bucket.createMultipartUpload is not implemented');
	}

	resumeMultipartUpload(_key: string, _uploadID: string): R2MultipartUpload {
		throw new Error('MemoryR2Bucket.resumeMultipartUpload is not implemented');
	}

	async delete(_keys: string | string[]): Promise<void> {
		throw new Error('MemoryR2Bucket.delete is not implemented');
	}

	async list(_options?: R2ListOptions): Promise<R2Objects> {
		throw new Error('MemoryR2Bucket.list is not implemented');
	}
}

function createMemoryR2Object(key: string, value: string): R2Object {
	return {
		key,
		version: 'test-version',
		size: value.length,
		etag: 'test',
		httpEtag: '"test"',
		checksums: {
			toJSON() {
				return {};
			}
		},
		uploaded: new Date(0),
		storageClass: 'Standard',
		writeHttpMetadata(headers: Headers) {
			headers.set('content-type', 'application/json');
		}
	};
}

function createMemoryR2ObjectBody(key: string, value: string): R2ObjectBody {
	return {
		...createMemoryR2Object(key, value),
		writeHttpMetadata(headers: Headers) {
			headers.set('content-type', 'application/json');
		},
		get body() {
			return new ReadableStream({
				start(controller) {
					controller.enqueue(new TextEncoder().encode(value));
					controller.close();
				}
			});
		},
		get bodyUsed() {
			return false;
		},
		async arrayBuffer() {
			return new TextEncoder().encode(value).buffer.slice(0);
		},
		async bytes() {
			return new TextEncoder().encode(value);
		},
		async text() {
			return value;
		},
		async json<T>(): Promise<T> {
			throw new Error('MemoryR2ObjectBody.json is not implemented');
		},
		async blob() {
			return new Blob([value]);
		}
	};
}
