export class FilesReadError extends Error {
	constructor(message: string, readonly status: number) {
		super(message);
		this.name = 'FilesReadError';
	}
}

export function isFilesAccessDenied(error: unknown): boolean {
	return error instanceof FilesReadError && (error.status === 401 || error.status === 403);
}
