import { supabaseOrganizationDirectory } from '$lib/organization/supabase-directory';
import type {
	CRMOrganizationPayload,
	CRMContactPayload,
	CRMActivityPayload,
	CRMOpportunityPayload,
	CRMPositionPayload,
	CRMTransitionPayload,
	CRMVocabulary
} from './crm-api-types';
import * as central from './crm-public-api';

export { CRMApiError } from './crm-error';

export function loadCRMOrganizationDirectory() {
	return supabaseOrganizationDirectory();
}

export function loadCRMData() {
	return central.loadSupabaseCRMData();
}

export function createCRMOrganization(payload: CRMOrganizationPayload) {
	return central.createSupabaseCRMOrganization(payload);
}

export function updateCRMOrganization(id: string, payload: CRMOrganizationPayload) {
	return central.updateSupabaseCRMOrganization(id, payload);
}

export function archiveCRMOrganization(id: string) {
	return central.archiveSupabaseCRMOrganization(id);
}

export function createCRMContact(payload: CRMContactPayload) {
	return central.createSupabaseCRMContact(payload);
}

export function updateCRMContact(id: string, payload: CRMContactPayload) {
	return central.updateSupabaseCRMContact(id, payload);
}

export function archiveCRMContact(id: string) {
	return central.archiveSupabaseCRMContact(id);
}

export function createCRMOpportunity(payload: CRMOpportunityPayload) {
	return central.createSupabaseCRMOpportunity(payload);
}

export function updateCRMOpportunity(id: string, payload: CRMOpportunityPayload) {
	return central.updateSupabaseCRMOpportunity(id, payload);
}

export function archiveCRMOpportunity(id: string) {
	return central.archiveSupabaseCRMOpportunity(id);
}

export function transitionCRMOpportunity(id: string, payload: CRMTransitionPayload) {
	return central.transitionSupabaseCRMOpportunity(id, payload);
}

export function positionCRMOpportunity(id: string, payload: CRMPositionPayload) {
	return central.positionSupabaseCRMOpportunity(id, payload);
}

export function createCRMActivity(payload: CRMActivityPayload) {
	return central.createSupabaseCRMActivity(payload);
}

export function updateCRMActivity(id: string, payload: CRMActivityPayload) {
	return central.updateSupabaseCRMActivity(id, payload);
}

export function saveCRMVocabulary(vocabulary: CRMVocabulary) {
	return central.saveSupabaseCRMVocabulary(vocabulary);
}
