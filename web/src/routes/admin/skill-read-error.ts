export class SkillReadError extends Error {
	constructor(message: string, readonly status: number) {
		super(message);
		this.name = 'SkillReadError';
	}
}

export function isSkillReadAccessDenied(error: unknown): boolean {
	return error instanceof SkillReadError && (error.status === 401 || error.status === 403);
}
