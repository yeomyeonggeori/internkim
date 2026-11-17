export const slowPlaneRequestMilliseconds = 1000;

export type SlowPlaneRequest = {
	method: string;
	path: string;
	status: number | 'failed';
	durationMilliseconds: number;
};

type Clock = () => number;

type PlaneFetch = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>;

type Report = (request: SlowPlaneRequest) => void;

function reportToTheLog(request: SlowPlaneRequest): void {
	console.warn('plane.slow_request', request);
}

function methodOf(input: RequestInfo | URL, init: RequestInit | undefined): string {
	if (init?.method) return init.method.toUpperCase();
	if (input instanceof Request) return input.method;
	return 'GET';
}

function pathOf(input: RequestInfo | URL): string {
	if (input instanceof Request) return new URL(input.url).pathname;
	return new URL(input).pathname;
}

export function timedPlaneFetch(
	underlying: PlaneFetch = fetch,
	report: Report = reportToTheLog,
	now: Clock = () => performance.now()
): typeof fetch {
	const timed = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
		const startedAt = now();
		const finish = (status: SlowPlaneRequest['status']): void => {
			const durationMilliseconds = Math.round(now() - startedAt);
			if (durationMilliseconds < slowPlaneRequestMilliseconds) return;
			report({ method: methodOf(input, init), path: pathOf(input), status, durationMilliseconds });
		};
		try {
			const response = await underlying(input, init);
			finish(response.status);
			return response;
		} catch (failure) {
			finish('failed');
			throw failure;
		}
	};
	return Object.assign(timed, { preconnect: fetch.preconnect });
}
