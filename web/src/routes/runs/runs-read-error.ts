export class RunsReadError extends Error {
	constructor(message: string, readonly status: number) {
		super(message);
		this.name = 'RunsReadError';
	}
}

export function isRunsAccessDenied(error: unknown): boolean {
	return error instanceof RunsReadError && (error.status === 401 || error.status === 403);
}
