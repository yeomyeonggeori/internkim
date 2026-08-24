import type { EmployeeLeaveErrorCode } from './employee-leave-types';

export class EmployeeLeaveAPIError extends Error {
	constructor(
		readonly code: EmployeeLeaveErrorCode | null,
		readonly status: number
	) {
		super('Employee leave API request failed');
		this.name = 'EmployeeLeaveAPIError';
	}
}
