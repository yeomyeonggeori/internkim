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

class FixedLengthStreamStandIn extends TransformStream<Uint8Array, Uint8Array> {
	constructor(expectedLength: number) {
		let seenLength = 0;
		super({
			transform(chunk, controller) {
				seenLength += chunk.byteLength;
				if (seenLength > expectedLength) controller.error(new TypeError(`the body is longer than ${expectedLength} bytes`));
				else controller.enqueue(chunk);
			},
			flush(controller) {
				if (seenLength !== expectedLength) controller.error(new TypeError(`the body ended at ${seenLength} of ${expectedLength} bytes`));
			}
		});
	}
}

Object.assign(globalThis, { FixedLengthStream: FixedLengthStreamStandIn });

plugin({
	name: 'cloudflare workers module',
	setup(build) {
		build.module('cloudflare:workers', () => ({
			exports: { WorkerEntrypoint: WorkerEntrypointStandIn },
			loader: 'object'
		}));
	}
});
