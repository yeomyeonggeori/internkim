import { describe, expect, test } from 'bun:test';
import { handleReleaseRegistryRequest } from '../../../../workers/release-registry/src/registry';

describe('Release Registry Worker', () => {
	test('rejects requests without release download token before reading R2', async () => {
		const bucket = new MemoryR2Bucket({ 'channels/stable.json': 'ok' });

		const response = await handleReleaseRegistryRequest(new Request('https://updates.intern.kim/channels/stable.json'), {
			RELEASE_BUCKET: bucket as unknown as R2Bucket,
			RELEASE_DOWNLOAD_TOKEN: 'download-token'
		});

		expect(response.status).toBe(401);
		expect(bucket.getCalls.length).toBe(0);
	});

	test('serves R2 object when release download token matches', async () => {
		const bucket = new MemoryR2Bucket({ 'channels/stable.json': '{"releaseID":"release-1"}' });

		const response = await handleReleaseRegistryRequest(
			new Request('https://updates.intern.kim/channels/stable.json', {
				headers: { 'X-InternKim-Release-Token': 'download-token' }
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

	test('rejects unsafe object keys', async () => {
		const bucket = new MemoryR2Bucket({ '../secret': 'secret' });

		const response = await handleReleaseRegistryRequest(
			new Request('https://updates.intern.kim/%252E%252E/secret', {
				headers: { 'X-InternKim-Release-Token': 'download-token' }
			}),
			{
				RELEASE_BUCKET: bucket as unknown as R2Bucket,
				RELEASE_DOWNLOAD_TOKEN: 'download-token'
			}
		);

		expect(response.status).toBe(404);
		expect(bucket.getCalls.length).toBe(0);
	});
});

class MemoryR2Bucket {
	getCalls: string[] = [];

	constructor(private readonly objects: Record<string, string>) {}

	async get(key: string) {
		this.getCalls.push(key);
		const value = this.objects[key];
		if (value === undefined) {
			return null;
		}
		return {
			body: value,
			size: value.length,
			httpEtag: '"test"',
			writeHttpMetadata(headers: Headers) {
				headers.set('content-type', 'application/json');
			}
		};
	}
}
