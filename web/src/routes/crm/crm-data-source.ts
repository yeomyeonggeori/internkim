import { isSupabaseConfigured } from '$lib/supabase';
import { supabaseOrganizationDirectory } from '$lib/organization/supabase-directory';
import * as device from './crm-api';
import { fetchCRMOrganizationDirectory } from './crm-directory';
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

export { CRMApiError } from './crm-api';

export function settlementIsConvertedByServer(): boolean {
	return isSupabaseConfigured();
}

export function loadCRMOrganizationDirectory() {
	return isSupabaseConfigured() ? supabaseOrganizationDirectory() : fetchCRMOrganizationDirectory();
}

export function loadCRMData() {
	return isSupabaseConfigured() ? central.loadSupabaseCRMData() : device.loadCRMData();
}

export function createCRMOrganization(payload: CRMOrganizationPayload) {
	return isSupabaseConfigured() ? central.createSupabaseCRMOrganization(payload) : device.createCRMOrganization(payload);
}

export function updateCRMOrganization(id: string, payload: CRMOrganizationPayload) {
	return isSupabaseConfigured() ? central.updateSupabaseCRMOrganization(id, payload) : device.updateCRMOrganization(id, payload);
}

export function archiveCRMOrganization(id: string) {
	return isSupabaseConfigured() ? central.archiveSupabaseCRMOrganization(id) : device.archiveCRMOrganization(id);
}

export function createCRMContact(payload: CRMContactPayload) {
	return isSupabaseConfigured() ? central.createSupabaseCRMContact(payload) : device.createCRMContact(payload);
}

export function updateCRMContact(id: string, payload: CRMContactPayload) {
	return isSupabaseConfigured() ? central.updateSupabaseCRMContact(id, payload) : device.updateCRMContact(id, payload);
}

export function archiveCRMContact(id: string) {
	return isSupabaseConfigured() ? central.archiveSupabaseCRMContact(id) : Promise.reject(new device.CRMApiError('contact archive is unavailable', 501, 'not_supported'));
}

export function createCRMOpportunity(payload: CRMOpportunityPayload) {
	return isSupabaseConfigured() ? central.createSupabaseCRMOpportunity(payload) : device.createCRMOpportunity(payload);
}

export function updateCRMOpportunity(id: string, payload: CRMOpportunityPayload) {
	return isSupabaseConfigured() ? central.updateSupabaseCRMOpportunity(id, payload) : device.updateCRMOpportunity(id, payload);
}

export function archiveCRMOpportunity(id: string) {
	return isSupabaseConfigured() ? central.archiveSupabaseCRMOpportunity(id) : device.archiveCRMOpportunity(id);
}

export function transitionCRMOpportunity(id: string, payload: CRMTransitionPayload) {
	return isSupabaseConfigured() ? central.transitionSupabaseCRMOpportunity(id, payload) : device.transitionCRMOpportunity(id, payload);
}

export function positionCRMOpportunity(id: string, payload: CRMPositionPayload) {
	return isSupabaseConfigured() ? central.positionSupabaseCRMOpportunity(id, payload) : device.positionCRMOpportunity(id, payload);
}

export function createCRMActivity(payload: CRMActivityPayload) {
	return isSupabaseConfigured() ? central.createSupabaseCRMActivity(payload) : device.createCRMActivity(payload);
}

export function updateCRMActivity(id: string, payload: CRMActivityPayload) {
	return isSupabaseConfigured() ? central.updateSupabaseCRMActivity(id, payload) : device.updateCRMActivity(id, payload);
}

export function saveCRMVocabulary(vocabulary: CRMVocabulary) {
	return isSupabaseConfigured()
		? central.saveSupabaseCRMVocabulary(vocabulary)
		: Promise.reject(new device.CRMApiError('CRM definitions are unavailable', 501, 'not_supported'));
}
