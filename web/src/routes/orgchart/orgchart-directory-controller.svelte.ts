import type { AdminPageText, OrgGroup, UserRecord, UsersResponse } from '../admin/admin-types';
import { apiErrorMessage, fetchAdminSession, saveOrgGroups, saveOrgProfiles } from '../admin/admin-api';
import { adminSessionRole, canManageOrgchart } from '../admin/admin-role-policy';
import { fetchOrgchartDirectory, orgchartApiErrorMessage } from './orgchart-api';
import { filterOrgchartRecords, orgchartFilterOptions, unassignedGroupID } from './orgchart-directory-model';
import { orgchartGroupSavePlan } from './orgchart-group-controller';
import {
	beginOrgchartProfileEdit,
	clearOrgchartProfileSaving,
	hasInvalidOrgchartSupervisor,
	hasUnsavedOrgchartProfileEdits,
	isOrgchartProfileChanged,
	isOrgchartProfileSaving,
	markOrgchartProfileSaving,
	normalizedOrgchartRecords,
	orgchartProfileSavePayload,
	orgchartProfileSnapshots,
	reconcileOrgchartProfileEdits,
	removeOrgchartProfileEdit,
	type OrgProfileSnapshot
} from './orgchart-profile-edit-controller';
import { orgchartOrganizationSections, type OrgchartOrganizationSection } from './orgchart-organization-model';
import type { orgchartDirectoryText } from './text';

type OrgchartDirectoryPageText = typeof orgchartDirectoryText.ko;

export class OrgchartDirectoryController {
	records = $state<UserRecord[]>([]);
	groups = $state<OrgGroup[]>([]);
	query = $state('');
	groupID = $state('');
	selectedUserID = $state('');
	editingUserID = $state('');
	isFilterOpen = $state(false);
	isAddingGroup = $state(false);
	isLoading = $state(true);
	isSavingGroups = $state(false);
	canManage = $state(false);
	errorMessage = $state('');
	newGroupName = $state('');
	originalProfiles = $state<Record<string, OrgProfileSnapshot>>({});
	editingRecordsByUserID = $state<Record<string, UserRecord>>({});
	savingProfileUserIDs = $state<Record<string, boolean>>({});

	filters = $derived({ query: this.query, groupID: this.groupID });
	options = $derived(orgchartFilterOptions(this.records, this.groups));
	visibleRecords = $derived(filterOrgchartRecords(this.records, this.filters));
	selectedRecord = $derived(this.records.find((record) => record.userID === this.selectedUserID));
	selectedEditingRecord = $derived(this.selectedRecord ? this.editingRecordsByUserID[this.selectedRecord.userID] : undefined);
	selectedRecordIsEditing = $derived(Boolean(this.selectedRecord && this.editingUserID === this.selectedRecord.userID && this.selectedEditingRecord));
	isSavingSelectedProfile = $derived(this.selectedRecord ? this.isSavingProfile(this.selectedRecord.userID) : false);
	hasInvalidSelectedSupervisor = $derived(this.selectedEditingRecord ? this.hasInvalidSupervisor(this.selectedEditingRecord) : false);

	private adminBaseURL: string;
	private text: OrgchartDirectoryPageText;
	private adminPageText: AdminPageText;

	constructor(adminBaseURL: string, text: OrgchartDirectoryPageText, adminPageText: AdminPageText) {
		this.adminBaseURL = adminBaseURL;
		this.text = text;
		this.adminPageText = adminPageText;
	}

	get organizationSections(): OrgchartOrganizationSection[] {
		return orgchartOrganizationSections(this.visibleRecords, this.groups, this.text.unassignedTeam);
	}

	async load(): Promise<void> {
		await Promise.all([this.loadDirectory(), this.loadAdminAccess()]);
	}

	async loadDirectory(): Promise<void> {
		this.isLoading = true;
		this.errorMessage = '';
		try {
			const response = await fetchOrgchartDirectory(this.text.loadError);
			this.applyUsersResponse(response);
			this.clearSelection();
		} catch (error) {
			this.errorMessage = orgchartApiErrorMessage(error, this.text.loadError);
		} finally {
			this.isLoading = false;
		}
	}

	async loadAdminAccess(): Promise<void> {
		try {
			const session = await fetchAdminSession(this.adminBaseURL, '');
			this.canManage = canManageOrgchart(adminSessionRole(session));
		} catch {
			this.canManage = false;
			this.isAddingGroup = false;
		}
	}

	selectGroup(value: string | undefined): void {
		const nextGroupID = value === allValue || value === undefined ? '' : value;
		if (nextGroupID === this.groupID) return;
		if (this.hasUnsavedProfileEdits()) {
			this.errorMessage = this.adminPageText.orgchart.unsavedChanges;
			return;
		}
		this.groupID = nextGroupID;
		this.clearSelection();
	}

	selectRecord(record: UserRecord): void {
		if (this.editingUserID && this.editingUserID !== record.userID && this.hasUnsavedProfileEdits()) {
			this.errorMessage = this.adminPageText.orgchart.unsavedChanges;
			return;
		}
		if (this.editingUserID && this.editingUserID !== record.userID) this.cancelProfileEdit(this.editingUserID);
		this.errorMessage = '';
		this.selectedUserID = record.userID;
	}

	clearSelection(): void {
		if (this.selectedUserID && this.editingUserID === this.selectedUserID && this.hasUnsavedProfileEdits()) {
			this.errorMessage = this.adminPageText.orgchart.unsavedChanges;
			return;
		}
		if (this.selectedUserID && this.editingUserID === this.selectedUserID) this.cancelProfileEdit(this.selectedUserID);
		this.selectedUserID = '';
	}

	editRecord(record: UserRecord): void {
		if (!this.canManage) return;
		this.errorMessage = '';
		this.selectedUserID = record.userID;
		this.editingUserID = record.userID;
		this.editingRecordsByUserID = beginOrgchartProfileEdit(this.editingRecordsByUserID, record);
	}

	cancelProfileEdit(userID: string): void {
		this.errorMessage = '';
		this.removeEditingRecord(userID);
		if (this.editingUserID === userID) this.editingUserID = '';
	}

	async saveProfile(userID: string): Promise<void> {
		const record = this.editingRecordsByUserID[userID];
		if (!record || !this.canManage || this.isSavingProfile(userID) || this.hasInvalidSupervisor(record)) return;
		if (!this.isChanged(record)) {
			this.cancelProfileEdit(userID);
			return;
		}
		this.savingProfileUserIDs = markOrgchartProfileSaving(this.savingProfileUserIDs, userID);
		this.errorMessage = '';
		try {
			this.applyUsersResponse(await saveOrgProfiles(this.adminBaseURL, [orgchartProfileSavePayload(record)], this.adminPageText.messages.userSaveError));
			this.removeEditingRecord(userID);
			this.editingUserID = '';
		} catch (error) {
			this.errorMessage = apiErrorMessage(error, this.adminPageText.messages.userSaveError);
		} finally {
			this.savingProfileUserIDs = clearOrgchartProfileSaving(this.savingProfileUserIDs, userID);
		}
	}

	async addGlobalGroup(): Promise<void> {
		const groupID = await this.addGroup(this.newGroupName);
		if (!groupID) return;
		this.newGroupName = '';
		this.isAddingGroup = false;
	}

	cancelAddGroup(): void {
		if (this.isSavingGroups) return;
		this.newGroupName = '';
		this.isAddingGroup = false;
	}

	handleAddGroupOpenChange(nextOpen: boolean): void {
		if (!nextOpen && this.isSavingGroups) return;
		this.isAddingGroup = nextOpen;
		if (!nextOpen) this.newGroupName = '';
	}

	toggleFilters(): void {
		this.isFilterOpen = !this.isFilterOpen;
	}

	private applyUsersResponse(response: UsersResponse, fallbackGroups: OrgGroup[] = []): void {
		const nextGroups = (response.availableGroups ?? fallbackGroups).map((group) => ({ ...group }));
		this.groups = nextGroups;
		if (!response.records) return;
		const records = normalizedOrgchartRecords(response.records);
		if (!records) return;
		this.records = records;
		this.originalProfiles = orgchartProfileSnapshots(this.records);
		this.editingRecordsByUserID = reconcileOrgchartProfileEdits(this.editingRecordsByUserID, this.records, nextGroups);
	}

	private isChanged(record: UserRecord): boolean {
		return isOrgchartProfileChanged(record, this.originalProfiles);
	}

	private isSavingProfile(userID: string): boolean {
		return isOrgchartProfileSaving(this.savingProfileUserIDs, userID);
	}

	private hasInvalidSupervisor(record: UserRecord): boolean {
		return hasInvalidOrgchartSupervisor(this.records, record);
	}

	private hasUnsavedProfileEdits(): boolean {
		return hasUnsavedOrgchartProfileEdits(this.editingRecordsByUserID, this.originalProfiles);
	}

	private removeEditingRecord(userID: string): void {
		this.editingRecordsByUserID = removeOrgchartProfileEdit(this.editingRecordsByUserID, userID);
	}

	private async addGroup(name: string): Promise<string> {
		const plan = orgchartGroupSavePlan(this.groups, name, () => crypto.randomUUID());
		if (!plan.groupID || !plan.shouldPersist) return plan.groupID;
		const isPersisted = await this.persistGroups(plan.groups);
		return isPersisted ? plan.groupID : '';
	}

	private async persistGroups(nextGroups: OrgGroup[]): Promise<boolean> {
		this.isSavingGroups = true;
		this.errorMessage = '';
		try {
			this.applyUsersResponse(await saveOrgGroups(this.adminBaseURL, nextGroups, this.adminPageText.messages.userSaveError), nextGroups);
			return true;
		} catch (error) {
			this.errorMessage = apiErrorMessage(error, this.adminPageText.messages.userSaveError);
			return false;
		} finally {
			this.isSavingGroups = false;
		}
	}

}

export const allValue = '__all__';
