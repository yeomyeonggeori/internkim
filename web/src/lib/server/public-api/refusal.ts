export type RefusalAnswer = {
	status: number;
	body: {
		error: string;
		errorCode: string;
		failureStage: string;
		retryable: boolean;
		safeRetry: boolean;
		[detail: string]: unknown;
	};
};

export abstract class Refusal extends Error {
	abstract readonly status: number;
	abstract readonly errorCode: string;
	abstract readonly failureStage: string;
	abstract readonly retryable: boolean;
	abstract readonly safeRetry: boolean;

	constructor(message: string) {
		super(message);
		this.name = new.target.name;
	}

	get details(): Record<string, unknown> {
		return {};
	}
}

export function answerOfRefusal(refusal: Refusal): RefusalAnswer {
	return {
		status: refusal.status,
		body: {
			error: refusal.message,
			errorCode: refusal.errorCode,
			failureStage: refusal.failureStage,
			retryable: refusal.retryable,
			safeRetry: refusal.safeRetry,
			...refusal.details
		}
	};
}

export class InputRefused extends Refusal {
	readonly status = 400;
	readonly failureStage = 'input_validation';
	readonly retryable = true;
	readonly safeRetry = true;

	constructor(message: string, readonly errorCode = 'invalid_input') {
		super(message);
	}
}

export class ToolOutOfReach extends Refusal {
	readonly status = 400;
	readonly errorCode = 'tool_out_of_reach';
	readonly failureStage = 'target_resolution';
	readonly retryable = false;
	readonly safeRetry = true;
}

export class CallerUnrecognized extends Refusal {
	readonly status = 401;
	readonly errorCode = 'unauthenticated';
	readonly failureStage = 'authorization';
	readonly retryable = false;
	readonly safeRetry = true;
}

export class AccessRefused extends Refusal {
	readonly status = 403;
	readonly failureStage = 'authorization';
	readonly retryable = false;
	readonly safeRetry = true;

	constructor(message: string, readonly errorCode = 'access_denied') {
		super(message);
	}
}

export class TargetMissing extends Refusal {
	readonly status = 404;
	readonly failureStage = 'target_resolution';
	readonly retryable = false;
	readonly safeRetry = true;

	constructor(message: string, readonly errorCode = 'not_found') {
		super(message);
	}
}

export class RecordMoved extends Refusal {
	readonly status = 409;
	readonly failureStage = 'execution';
	readonly retryable = true;
	readonly safeRetry = true;

	constructor(message: string, readonly errorCode = 'conflict') {
		super(message);
	}
}

export class RecordAlreadyHolds extends Refusal {
	readonly status = 409;
	readonly failureStage = 'execution';
	readonly retryable = false;
	readonly safeRetry = true;

	constructor(message: string, readonly errorCode = 'record_duplicate') {
		super(message);
	}
}

export class RecordRuleBroken extends Refusal {
	readonly status = 422;
	readonly errorCode = 'record_refused';
	readonly failureStage = 'execution';
	readonly retryable = false;
	readonly safeRetry = true;
}

export class ServiceUnanswered extends Refusal {
	readonly status = 502;
	readonly failureStage = 'execution';
	readonly retryable = true;
	readonly safeRetry = false;

	constructor(message: string, readonly errorCode = 'unavailable') {
		super(message);
	}
}

export type PostgresError = { message: string; code?: string };

const insufficientPrivilege = '42501';
const serializationFailure = '40001';
const uniqueViolation = '23505';
const noDataFound = 'P0002';

export function refusalOfPostgresError(error: PostgresError, whenForbidden = error.message): Refusal {
	if (error.code === insufficientPrivilege) return new AccessRefused(whenForbidden);
	if (error.code === serializationFailure) return new RecordMoved(error.message);
	if (error.code === uniqueViolation) return new RecordAlreadyHolds(error.message);
	if (error.code === noDataFound) return new TargetMissing(error.message);
	return new RecordRuleBroken(error.message);
}

export type ServiceError = { message: string; status?: number };

export function refusalOfServiceError(error: ServiceError, message = error.message): Refusal {
	if (error.status === 400) return new InputRefused(message);
	if (error.status === 403) return new AccessRefused(message);
	if (error.status === 404) return new TargetMissing(message);
	return new ServiceUnanswered(message);
}
