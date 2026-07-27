import type { OrgProfileUpdate } from '../admin/admin-api';
import { copyUserRecord, reconcileEditingRecords } from '../admin/organization-editing-records';
import {
	isOrgProfileChanged,
	normalizeOrgProfileRecord,
	orgProfileSnapshot,
	orgProfileUpdate,
	type OrgProfileSnapshot
} from '../admin/organization-profile-model';
import { isSupervisorCandidateForRecord } from '../admin/organization-tree';
import type { OrgGroup, UserRecord } from '../../lib/organization/types';

export type { OrgProfileSnapshot };

export type OrganizationProfileSavingState = Record<string, boolean>;

export function normalizedOrganizationRecords(records: UserRecord[] | undefined): UserRecord[] | undefined {
	return records?.map(normalizeOrgProfileRecord);
}

export function organizationProfileSnapshots(records: UserRecord[]): Record<string, OrgProfileSnapshot> {
	return Object.fromEntries(records.map((record) => [record.userID, orgProfileSnapshot(record)]));
}

export function reconcileOrganizationProfileEdits(editingRecordsByUserID: Record<string, UserRecord>, records: UserRecord[], groups: OrgGroup[]): Record<string, UserRecord> {
	return reconcileEditingRecords(editingRecordsByUserID, records, groups);
}

export function beginOrganizationProfileEdit(editingRecordsByUserID: Record<string, UserRecord>, record: UserRecord): Record<string, UserRecord> {
	if (editingRecordsByUserID[record.userID]) return editingRecordsByUserID;
	return {
		...editingRecordsByUserID,
		[record.userID]: copyUserRecord(record)
	};
}

export function removeOrganizationProfileEdit(editingRecordsByUserID: Record<string, UserRecord>, userID: string): Record<string, UserRecord> {
	return Object.fromEntries(Object.entries(editingRecordsByUserID).filter(([candidateUserID]) => candidateUserID !== userID));
}

export function isOrganizationProfileChanged(record: UserRecord, originalProfiles: Record<string, OrgProfileSnapshot>): boolean {
	return isOrgProfileChanged(record, originalProfiles[record.userID]);
}

export function hasUnsavedOrganizationProfileEdits(editingRecordsByUserID: Record<string, UserRecord>, originalProfiles: Record<string, OrgProfileSnapshot>): boolean {
	return Object.values(editingRecordsByUserID).some((record) => isOrganizationProfileChanged(record, originalProfiles));
}

export function isOrganizationProfileSaving(savingProfileUserIDs: OrganizationProfileSavingState, userID: string): boolean {
	return savingProfileUserIDs[userID] === true;
}

export function markOrganizationProfileSaving(savingProfileUserIDs: OrganizationProfileSavingState, userID: string): OrganizationProfileSavingState {
	return { ...savingProfileUserIDs, [userID]: true };
}

export function clearOrganizationProfileSaving(savingProfileUserIDs: OrganizationProfileSavingState, userID: string): OrganizationProfileSavingState {
	return Object.fromEntries(Object.entries(savingProfileUserIDs).filter(([candidateUserID]) => candidateUserID !== userID));
}

export function hasInvalidOrganizationSupervisor(records: UserRecord[], record: UserRecord): boolean {
	return !isSupervisorCandidateForRecord(records, record);
}

export function organizationProfileSavePayload(record: UserRecord): OrgProfileUpdate {
	return orgProfileUpdate(record);
}
