import { plugin } from 'bun';

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
