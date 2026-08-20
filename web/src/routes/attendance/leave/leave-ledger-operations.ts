import type { AttendanceText } from '../text';

type LeaveText = AttendanceText['leave'];

export const requestDerivedLedgerOperations = [
	'reserve',
	'release',
	'restore',
	'use',
	'untrackedUse'
] as const;

export const balanceOwnLedgerOperations = [
	'grant',
	'expire',
	'carryover',
	'legalCorrection',
	'adjustment'
] as const;

const requestDerived = new Set<string>(requestDerivedLedgerOperations);

export function isBalanceOwnLedgerOperation(operationType: string): boolean {
	return !requestDerived.has(operationType);
}

export function ledgerOperationLabel(text: LeaveText, operationType: string): string {
	switch (operationType) {
		case 'grant':
			return text.operationGrant;
		case 'expire':
			return text.operationExpire;
		case 'carryover':
			return text.operationCarryover;
		case 'legalCorrection':
			return text.operationLegalCorrection;
		case 'adjustment':
			return text.operationAdjustment;
		case 'reserve':
			return text.operationReserve;
		case 'release':
			return text.operationRelease;
		case 'restore':
			return text.operationRestore;
		case 'use':
		case 'untrackedUse':
			return text.operationUse;
		default:
			return text.operationOther;
	}
}
