import type { OrgProfileUpdate } from '../admin/admin-api';
import { copyUserRecord, reconcileEditingRecords } from '../admin/orgchart-editing-records';
import {
	isOrgProfileChanged,
	normalizeOrgProfileRecord,
	orgProfileSnapshot,
	orgProfileUpdate,
	type OrgProfileSnapshot
} from '../admin/orgchart-profile-model';
import { isSupervisorCandidateForRecord } from '../admin/orgchart-tree';
import type { OrgGroup, UserRecord } from '../../lib/orgchart/types';

export type { OrgProfileSnapshot };

export type OrgchartProfileSavingState = Record<string, boolean>;

export function normalizedOrgchartRecords(records: UserRecord[] | undefined): UserRecord[] | undefined {
	return records?.map(normalizeOrgProfileRecord);
}

export function orgchartProfileSnapshots(records: UserRecord[]): Record<string, OrgProfileSnapshot> {
	return Object.fromEntries(records.map((record) => [record.userID, orgProfileSnapshot(record)]));
}

export function reconcileOrgchartProfileEdits(editingRecordsByUserID: Record<string, UserRecord>, records: UserRecord[], groups: OrgGroup[]): Record<string, UserRecord> {
	return reconcileEditingRecords(editingRecordsByUserID, records, groups);
}

export function beginOrgchartProfileEdit(editingRecordsByUserID: Record<string, UserRecord>, record: UserRecord): Record<string, UserRecord> {
	if (editingRecordsByUserID[record.userID]) return editingRecordsByUserID;
	return {
		...editingRecordsByUserID,
		[record.userID]: copyUserRecord(record)
	};
}

export function removeOrgchartProfileEdit(editingRecordsByUserID: Record<string, UserRecord>, userID: string): Record<string, UserRecord> {
	return Object.fromEntries(Object.entries(editingRecordsByUserID).filter(([candidateUserID]) => candidateUserID !== userID));
}

export function isOrgchartProfileChanged(record: UserRecord, originalProfiles: Record<string, OrgProfileSnapshot>): boolean {
	return isOrgProfileChanged(record, originalProfiles[record.userID]);
}

export function hasUnsavedOrgchartProfileEdits(editingRecordsByUserID: Record<string, UserRecord>, originalProfiles: Record<string, OrgProfileSnapshot>): boolean {
	return Object.values(editingRecordsByUserID).some((record) => isOrgchartProfileChanged(record, originalProfiles));
}

export function isOrgchartProfileSaving(savingProfileUserIDs: OrgchartProfileSavingState, userID: string): boolean {
	return savingProfileUserIDs[userID] === true;
}

export function markOrgchartProfileSaving(savingProfileUserIDs: OrgchartProfileSavingState, userID: string): OrgchartProfileSavingState {
	return { ...savingProfileUserIDs, [userID]: true };
}

export function clearOrgchartProfileSaving(savingProfileUserIDs: OrgchartProfileSavingState, userID: string): OrgchartProfileSavingState {
	return Object.fromEntries(Object.entries(savingProfileUserIDs).filter(([candidateUserID]) => candidateUserID !== userID));
}

export function hasInvalidOrgchartSupervisor(records: UserRecord[], record: UserRecord): boolean {
	return !isSupervisorCandidateForRecord(records, record);
}

export function orgchartProfileSavePayload(record: UserRecord): OrgProfileUpdate {
	return orgProfileUpdate(record);
}
