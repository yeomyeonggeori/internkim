import { plugin } from 'bun';
import { timingSafeEqual } from 'node:crypto';

Object.assign(crypto.subtle, {
	timingSafeEqual: (first: Uint8Array, second: Uint8Array) => timingSafeEqual(first, second)
});

class WorkerEntrypointStandIn {
	constructor(
		protected readonly ctx: unknown,
		protected readonly env: unknown
	) {}
}

plugin({
	name: 'cloudflare workers module',
	setup(build) {
		build.module('cloudflare:workers', () => ({
			exports: { WorkerEntrypoint: WorkerEntrypointStandIn },
			loader: 'object'
		}));
	}
});
