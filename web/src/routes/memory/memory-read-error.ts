import { ToolRefused } from '$lib/tool-answer';

export class MemoryReadError extends Error {
	constructor(message: string, readonly status: number) {
		super(message);
		this.name = 'MemoryReadError';
	}
}

export function isMemoryAccessDenied(error: unknown): boolean {
	return (error instanceof MemoryReadError || error instanceof ToolRefused) && (error.status === 401 || error.status === 403);
}
