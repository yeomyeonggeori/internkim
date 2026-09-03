export class CRMApiError extends Error {
	constructor(
		message: string,
		readonly status: number,
		readonly code: string
	) {
		super(message);
		this.name = 'CRMApiError';
	}
}
