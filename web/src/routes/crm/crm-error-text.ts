import { CRMApiError } from './crm-api';
import { CRMOwnerResolutionError } from './crm-mappers';
import { CRMOpportunityTransitionError } from './crm-opportunity-transition';

export type CRMErrorText = {
	organizationLoadFailed: string;
	refreshAfterSaveFailed: string;
	permissionDenied: string;
	invalidRequest: string;
	recordNotFound: string;
	conflict: string;
	processingFailed: string;
	ownerNotFound: string;
	ownerAmbiguous: string;
	lostReasonRequired: string;
	baseCurrencyConversionRequired: string;
	definitionInUse: string;
};

export class CRMPageError extends Error {
	constructor(readonly code: 'organization_load_failed' | 'refresh_after_save_failed') {
		super(code);
		this.name = 'CRMPageError';
	}
}

export function crmErrorMessage(error: unknown, text: CRMErrorText): string {
	if (error instanceof CRMPageError) {
		return error.code === 'organization_load_failed' ? text.organizationLoadFailed : text.refreshAfterSaveFailed;
	}
	if (error instanceof CRMOpportunityTransitionError) {
		return error.code === 'lost_reason_required' ? text.lostReasonRequired : text.baseCurrencyConversionRequired;
	}
	if (error instanceof CRMOwnerResolutionError) {
		return error.code === 'owner_not_found' ? text.ownerNotFound : text.ownerAmbiguous;
	}
	if (error instanceof CRMApiError) {
		if (error.code === '2BP01') return text.definitionInUse;
		if (error.status === 401 || error.status === 403) return text.permissionDenied;
		if (error.status === 400) return text.invalidRequest;
		if (error.status === 404) return text.recordNotFound;
		if (error.status === 409) return text.conflict;
		return text.processingFailed;
	}
	if (error instanceof Error && error.message) return error.message;
	return text.processingFailed;
}
