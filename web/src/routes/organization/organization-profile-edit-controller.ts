import { copyUserRecord, reconcileEditingRecords } from '../admin/organization-editing-records';
import {
	isOrgProfileChanged,
	normalizeOrgProfileRecord,
	orgProfileSnapshot,
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
	return Object.fromEntries(records.map((record) => [record.memberID, orgProfileSnapshot(record)]));
}

export function reconcileOrganizationProfileEdits(editingRecordsByUserID: Record<string, UserRecord>, records: UserRecord[], groups: OrgGroup[]): Record<string, UserRecord> {
	return reconcileEditingRecords(editingRecordsByUserID, records, groups);
}

export function beginOrganizationProfileEdit(editingRecordsByUserID: Record<string, UserRecord>, record: UserRecord): Record<string, UserRecord> {
	if (editingRecordsByUserID[record.memberID]) return editingRecordsByUserID;
	return {
		...editingRecordsByUserID,
		[record.memberID]: copyUserRecord(record)
	};
}

export function removeOrganizationProfileEdit(editingRecordsByUserID: Record<string, UserRecord>, memberID: string): Record<string, UserRecord> {
	return Object.fromEntries(Object.entries(editingRecordsByUserID).filter(([candidateUserID]) => candidateUserID !== memberID));
}

export function isOrganizationProfileChanged(record: UserRecord, originalProfiles: Record<string, OrgProfileSnapshot>): boolean {
	return isOrgProfileChanged(record, originalProfiles[record.memberID]);
}

export function hasUnsavedOrganizationProfileEdits(editingRecordsByUserID: Record<string, UserRecord>, originalProfiles: Record<string, OrgProfileSnapshot>): boolean {
	return Object.values(editingRecordsByUserID).some((record) => isOrganizationProfileChanged(record, originalProfiles));
}

export function isOrganizationProfileSaving(savingProfileUserIDs: OrganizationProfileSavingState, memberID: string): boolean {
	return savingProfileUserIDs[memberID] === true;
}

export function markOrganizationProfileSaving(savingProfileUserIDs: OrganizationProfileSavingState, memberID: string): OrganizationProfileSavingState {
	return { ...savingProfileUserIDs, [memberID]: true };
}

export function clearOrganizationProfileSaving(savingProfileUserIDs: OrganizationProfileSavingState, memberID: string): OrganizationProfileSavingState {
	return Object.fromEntries(Object.entries(savingProfileUserIDs).filter(([candidateUserID]) => candidateUserID !== memberID));
}

export function hasInvalidOrganizationSupervisor(records: UserRecord[], record: UserRecord): boolean {
	return !isSupervisorCandidateForRecord(records, record);
}
