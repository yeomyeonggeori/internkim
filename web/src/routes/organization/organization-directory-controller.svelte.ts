import type { AdminPageText } from '../admin/admin-types';
import type { Locale } from '../../lib/i18n/locale.svelte';
import { apiErrorMessage, fetchAdminSession, saveOrgGroups, saveOrgProfiles } from '../admin/admin-api';
import { adminSessionRole, canManageOrganization } from '../admin/admin-role-policy';
import { fetchOrganizationDirectory, fetchWebSessionEmail, organizationApiErrorMessage, saveOwnPhoneNumber } from './organization-api';
import { filterOrganizationRecords, organizationFilterOptions, unassignedGroupID } from './organization-directory-model';
import { organizationGroupSavePlan } from './organization-group-controller';
import { OrganizationOrganizationEditController } from './organization-edit-controller.svelte';
import { organizationOrganizationTree, type OrganizationOrganizationTree } from './organization-tree-model';
import {
	beginOrganizationProfileEdit,
	clearOrganizationProfileSaving,
	hasInvalidOrganizationSupervisor,
	hasUnsavedOrganizationProfileEdits,
	isOrganizationProfileChanged,
	isOrganizationProfileSaving,
	markOrganizationProfileSaving,
	normalizedOrganizationRecords,
	organizationProfileSavePayload,
	organizationProfileSnapshots,
	reconcileOrganizationProfileEdits,
	removeOrganizationProfileEdit,
	type OrgProfileSnapshot
} from './organization-profile-edit-controller';
import { organizationOrganizationSections, type OrganizationOrganizationSection } from './organization-model';
import type { OrgGroup, UserRecord, UsersResponse } from '../../lib/organization/types';
import type { organizationDirectoryText } from './text';

type OrganizationDirectoryPageText = typeof organizationDirectoryText.ko;

export class OrganizationDirectoryController {
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
	hasExpiredAdminSession = $state(false);
	sessionEmail = $state('');
	isSavingOwnPhoneNumber = $state(false);
	errorMessage = $state('');
	newGroupName = $state('');
	newGroupParentID = $state('');
	originalProfiles = $state<Record<string, OrgProfileSnapshot>>({});
	editingRecordsByUserID = $state<Record<string, UserRecord>>({});
	savingProfileUserIDs = $state<Record<string, boolean>>({});
	organizationEdit = new OrganizationOrganizationEditController();

	options = $derived(organizationFilterOptions(this.records, this.groups));
	selectedRecord = $derived(this.records.find((record) => record.userID === this.selectedUserID));
	selectedEditingRecord = $derived(this.selectedRecord ? this.editingRecordsByUserID[this.selectedRecord.userID] : undefined);
	selectedRecordIsEditing = $derived(Boolean(this.selectedRecord && this.editingUserID === this.selectedRecord.userID && this.selectedEditingRecord));
	isSavingSelectedProfile = $derived(this.selectedRecord ? this.isSavingProfile(this.selectedRecord.userID) : false);
	hasInvalidSelectedSupervisor = $derived(this.selectedEditingRecord ? this.hasInvalidSupervisor(this.selectedEditingRecord) : false);

	private adminBaseURL: string;
	private text: OrganizationDirectoryPageText;
	private adminPageText: AdminPageText;
	private resolveLocale: () => Locale;

	constructor(adminBaseURL: string, text: OrganizationDirectoryPageText, adminPageText: AdminPageText, resolveLocale: () => Locale = () => 'ko') {
		this.adminBaseURL = adminBaseURL;
		this.text = text;
		this.adminPageText = adminPageText;
		this.resolveLocale = resolveLocale;
	}

	get organizationSections(): OrganizationOrganizationSection[] {
		const records = filterOrganizationRecords(this.records, { query: this.query, groupID: this.groupID === unassignedGroupID ? unassignedGroupID : '' });
		if (this.groupID === unassignedGroupID) return organizationOrganizationSections(records, [], this.text.unassignedTeam);
		return organizationOrganizationSections(records, this.activeGroups, this.text.allOrganizations, this.groupID, this.records, this.resolveLocale());
	}

	get activeGroups(): OrgGroup[] {
		return this.organizationEdit.isEditing ? this.organizationEdit.draftGroups : this.groups;
	}

	get organizationTree(): OrganizationOrganizationTree {
		return organizationOrganizationTree(this.activeGroups, this.records, this.text.allOrganizations);
	}

	get selectedOrganizationName(): string {
		if (!this.groupID) return this.text.allOrganizations;
		if (this.groupID === unassignedGroupID) return this.text.unassignedTeam;
		return this.activeGroups.find((group) => group.id === this.groupID)?.name ?? this.text.allOrganizations;
	}

	async load(): Promise<void> {
		await Promise.all([this.loadDirectory(), this.loadAdminAccess()]);
	}

	async loadDirectory(): Promise<void> {
		this.isLoading = true;
		this.errorMessage = '';
		try {
			const response = await fetchOrganizationDirectory(this.text.loadError);
			this.applyUsersResponse(response);
			this.clearSelection();
		} catch (error) {
			this.errorMessage = organizationApiErrorMessage(error, this.text.loadError);
		} finally {
			this.isLoading = false;
		}
	}

	async loadAdminAccess(): Promise<void> {
		this.sessionEmail = await fetchWebSessionEmail();
		try {
			const session = await fetchAdminSession(this.adminBaseURL, '');
			this.canManage = canManageOrganization(adminSessionRole(session));
			this.hasExpiredAdminSession = false;
		} catch {
			this.canManage = false;
			this.isAddingGroup = false;
			this.hasExpiredAdminSession = Boolean(this.sessionEmail);
		}
	}

	isOwnRecord(record: UserRecord | undefined): boolean {
		if (!record || !this.sessionEmail) return false;
		return record.email.trim().toLowerCase() === this.sessionEmail;
	}

	async saveOwnPhoneNumber(phoneNumber: string): Promise<void> {
		const record = this.selectedRecord;
		if (!record || !this.isOwnRecord(record)) return;
		this.isSavingOwnPhoneNumber = true;
		try {
			const savedPhoneNumber = await saveOwnPhoneNumber(phoneNumber);
			this.records = this.records.map((candidate) =>
				candidate.userID === record.userID ? { ...candidate, phoneNumber: savedPhoneNumber } : candidate
			);
			this.errorMessage = '';
		} catch (error) {
			this.errorMessage = organizationApiErrorMessage(error, this.text.loadError);
		} finally {
			this.isSavingOwnPhoneNumber = false;
		}
	}

	selectGroup(value: string | undefined): void {
		if (this.organizationEdit.isEditing) return;
		const nextGroupID = value === allValue || value === undefined ? '' : value;
		if (nextGroupID === this.groupID) return;
		if (this.hasUnsavedProfileEdits()) {
			this.errorMessage = this.adminPageText.organization.unsavedChanges;
			return;
		}
		this.groupID = nextGroupID;
		this.clearSelection();
	}

	selectRecord(record: UserRecord): void {
		if (this.organizationEdit.isEditing) return;
		if (this.editingUserID && this.editingUserID !== record.userID && this.hasUnsavedProfileEdits()) {
			this.errorMessage = this.adminPageText.organization.unsavedChanges;
			return;
		}
		if (this.editingUserID && this.editingUserID !== record.userID) this.cancelProfileEdit(this.editingUserID);
		this.errorMessage = '';
		this.selectedUserID = record.userID;
	}

	clearSelection(): void {
		if (this.selectedUserID && this.editingUserID === this.selectedUserID && this.hasUnsavedProfileEdits()) {
			this.errorMessage = this.adminPageText.organization.unsavedChanges;
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
		this.editingRecordsByUserID = beginOrganizationProfileEdit(this.editingRecordsByUserID, record);
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
		this.savingProfileUserIDs = markOrganizationProfileSaving(this.savingProfileUserIDs, userID);
		this.errorMessage = '';
		try {
			this.applyUsersResponse(await saveOrgProfiles(this.adminBaseURL, [organizationProfileSavePayload(record)], this.adminPageText.messages.userSaveError));
			this.removeEditingRecord(userID);
			this.editingUserID = '';
		} catch (error) {
			this.errorMessage = apiErrorMessage(error, this.adminPageText.messages.userSaveError);
		} finally {
			this.savingProfileUserIDs = clearOrganizationProfileSaving(this.savingProfileUserIDs, userID);
		}
	}

	async addGlobalGroup(): Promise<void> {
		const groupID = await this.addGroup(this.newGroupName, this.newGroupParentID);
		if (!groupID) return;
		this.newGroupName = '';
		this.newGroupParentID = '';
		this.isAddingGroup = false;
	}

	cancelAddGroup(): void {
		if (this.isSavingGroups) return;
		this.newGroupName = '';
		this.newGroupParentID = '';
		this.isAddingGroup = false;
	}

	handleAddGroupOpenChange(nextOpen: boolean): void {
		if (!nextOpen && this.isSavingGroups) return;
		this.isAddingGroup = nextOpen;
		if (!nextOpen) {
			this.newGroupName = '';
			this.newGroupParentID = '';
		}
	}

	beginOrganizationEdit(): void {
		if (!this.canManage || this.organizationEdit.isEditing) return;
		if (this.hasUnsavedProfileEdits()) {
			this.errorMessage = this.adminPageText.organization.unsavedChanges;
			return;
		}
		this.errorMessage = '';
		this.selectedUserID = '';
		this.isAddingGroup = false;
		this.organizationEdit.begin(this.groups);
	}

	cancelOrganizationEdit(): void {
		if (this.isSavingGroups) return;
		this.organizationEdit.cancel();
	}

	moveOrganization(groupID: string, insertionIndex: number, depth: number): void {
		this.organizationEdit.move(groupID, insertionIndex, depth);
	}

	async saveOrganizationEdit(): Promise<void> {
		if (!this.organizationEdit.isEditing || this.isSavingGroups) return;
		if (await this.persistGroups(this.organizationEdit.draftGroups)) this.organizationEdit.cancel();
	}

	private applyUsersResponse(response: UsersResponse, fallbackGroups: OrgGroup[] = []): void {
		const nextGroups = (response.availableGroups ?? fallbackGroups).map((group) => ({ ...group }));
		this.groups = nextGroups;
		if (!response.records) return;
		const records = normalizedOrganizationRecords(response.records);
		if (!records) return;
		this.records = records;
		this.originalProfiles = organizationProfileSnapshots(this.records);
		this.editingRecordsByUserID = reconcileOrganizationProfileEdits(this.editingRecordsByUserID, this.records, nextGroups);
	}

	private isChanged(record: UserRecord): boolean {
		return isOrganizationProfileChanged(record, this.originalProfiles);
	}

	private isSavingProfile(userID: string): boolean {
		return isOrganizationProfileSaving(this.savingProfileUserIDs, userID);
	}

	private hasInvalidSupervisor(record: UserRecord): boolean {
		return hasInvalidOrganizationSupervisor(this.records, record);
	}

	private hasUnsavedProfileEdits(): boolean {
		return hasUnsavedOrganizationProfileEdits(this.editingRecordsByUserID, this.originalProfiles);
	}

	private removeEditingRecord(userID: string): void {
		this.editingRecordsByUserID = removeOrganizationProfileEdit(this.editingRecordsByUserID, userID);
	}

	private async addGroup(name: string, parentID: string): Promise<string> {
		const plan = organizationGroupSavePlan(this.groups, name, parentID, () => crypto.randomUUID());
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
