import { EmployeeLeaveAPIError } from './employee-leave-api';
import type { EmployeeLeaveErrorCode } from './employee-leave-types';

type EmployeeLeaveErrorText = {
	errorInvalidInput: string;
	errorLeaveConflict: string;
	errorWorkConflict: string;
	errorInsufficientBalance: string;
	errorHireDateRequired?: string;
	errorInvalidAttachment: string;
	errorRequestNotFound: string;
	errorInvalidStatus: string;
	errorLegacyMigrationStale?: string;
	errorLegacyMigrationConflict?: string;
	errorInternal: string;
};

export function employeeLeaveErrorMessage(
	error: unknown,
	text: EmployeeLeaveErrorText,
	fallbackMessage: string
): string {
	if (!(error instanceof EmployeeLeaveAPIError) || !error.code) return fallbackMessage;
	const messages: Record<EmployeeLeaveErrorCode, string> = {
		invalidInput: text.errorInvalidInput,
		leaveConflict: text.errorLeaveConflict,
		workConflict: text.errorWorkConflict,
		insufficientBalance: text.errorInsufficientBalance,
		hireDateRequired: text.errorHireDateRequired ?? fallbackMessage,
		invalidAttachment: text.errorInvalidAttachment,
		requestNotFound: text.errorRequestNotFound,
		invalidStatus: text.errorInvalidStatus,
		legacyMigrationStale: text.errorLegacyMigrationStale ?? fallbackMessage,
		legacyMigrationConflict: text.errorLegacyMigrationConflict ?? fallbackMessage,
		internal: text.errorInternal
	};
	return messages[error.code];
}
