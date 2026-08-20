import type { AttendanceText } from '../text';

type LeaveText = AttendanceText['leave'];

const balanceOwnOperations = new Set(['grant', 'expire', 'adjustment', 'adminAdjustment']);

export function isBalanceOwnLedgerOperation(operationType: string): boolean {
	return balanceOwnOperations.has(operationType);
}

export function ledgerOperationLabel(text: LeaveText, operationType: string): string {
	switch (operationType) {
		case 'grant':
			return text.operationGrant;
		case 'expire':
			return text.operationExpire;
		case 'adjustment':
		case 'adminAdjustment':
			return text.operationAdjustment;
		case 'reserve':
			return text.operationReserve;
		case 'release':
			return text.operationRelease;
		case 'restore':
			return text.operationRestore;
		case 'use':
			return text.operationUse;
		default:
			return text.operationOther;
	}
}
