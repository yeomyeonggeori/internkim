import { describe, expect, test } from 'bun:test';
import { timedPlaneFetch, type SlowPlaneRequest } from '$lib/server/timed-plane-fetch';

function clockAdvancing(steps: number[]): () => number {
	let index = 0;
	return () => steps[Math.min(index++, steps.length - 1)];
}

describe('a request to the plane', () => {
	test('is reported with its method, path and status when it takes a second or more', async () => {
		const reported: SlowPlaneRequest[] = [];
		const timed = timedPlaneFetch(
			async () => new Response('{}', { status: 201 }),
			(request) => reported.push(request),
			clockAdvancing([0, 1500])
		);

		await timed('https://plane.example.com/auth/v1/admin/users?email=sample%40example.com', { method: 'post' });

		expect(reported).toEqual([
			{ method: 'POST', path: '/auth/v1/admin/users', status: 201, durationMilliseconds: 1500 }
		]);
	});

	test('is not reported when it answers within a second', async () => {
		const reported: SlowPlaneRequest[] = [];
		const timed = timedPlaneFetch(
			async () => new Response('{}'),
			(request) => reported.push(request),
			clockAdvancing([0, 999])
		);

		await timed(new URL('https://plane.example.com/rest/v1/member'));

		expect(reported).toEqual([]);
	});

	test('is reported as failed and still throws when it never answers', async () => {
		const reported: SlowPlaneRequest[] = [];
		const timed = timedPlaneFetch(
			async () => {
				throw new TypeError('network down');
			},
			(request) => reported.push(request),
			clockAdvancing([0, 15000])
		);

		await expect(timed(new Request('https://plane.example.com/rest/v1/rpc/person_set', { method: 'POST' }))).rejects.toThrow(
			'network down'
		);
		expect(reported).toEqual([
			{ method: 'POST', path: '/rest/v1/rpc/person_set', status: 'failed', durationMilliseconds: 15000 }
		]);
	});
});
