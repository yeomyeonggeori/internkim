<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { apiErrorMessage, fetchUsers, saveOrgGroups, saveOrgProfiles } from './admin-api';
	import type { AdminPageText, OrgGroup, UserRecord, UsersResponse } from './admin-types';
	import { copyUserRecord, reconcileEditingRecords } from './orgchart-editing-records';
	import OrgchartProfileCard from './orgchart-profile-card.svelte';
	import {
		isOrgProfileChanged,
		normalizeOrgProfileRecord,
		orgProfileSnapshot,
		orgProfileUpdate,
		type OrgProfileSnapshot
	} from './orgchart-profile-model';
	import { isSupervisorCandidateForRecord, orgForest, type OrgNode } from './orgchart-tree';

	type OrgchartSectionProps = {
		adminBaseURL: string;
		fleetID: string;
		isDeviceContext: boolean;
		text: AdminPageText;
	};

	let { adminBaseURL, fleetID, isDeviceContext, text }: OrgchartSectionProps = $props();

	let loadedAdminBaseURL = $state('');
	let userRecords = $state<UserRecord[]>([]);
	let groups = $state<OrgGroup[]>([]);
	let originalProfiles = $state<Record<string, OrgProfileSnapshot>>({});
	let isEditing = $state(false);
	let editingRecordsByUserID = $state<Record<string, UserRecord>>({});
	let isLoading = $state(false);
	let isSavingGroups = $state(false);
	let savingProfileUserIDs = $state<Record<string, boolean>>({});
	let errorMessage = $state('');
	let newGroupName = $state('');

	function isChanged(record: UserRecord) {
		return isOrgProfileChanged(record, originalProfiles[record.userID]);
	}

	function isSavingProfile(userID: string) {
		return savingProfileUserIDs[userID] === true;
	}

	function hasInvalidSupervisor(record: UserRecord) {
		return !isSupervisorCandidateForRecord(userRecords, record);
	}

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		loadUsers();
	});

	$effect(() => {
		if (isEditing) return;
		editingRecordsByUserID = {};
	});

	function applyUsersResponse(response: UsersResponse) {
		groups = (response.availableGroups ?? []).map((group) => ({ ...group }));
		userRecords = (response.records ?? []).map(normalizeOrgProfileRecord);
		originalProfiles = Object.fromEntries(userRecords.map((record) => [record.userID, orgProfileSnapshot(record)]));
	}

	function applyGroupsResponse(response: UsersResponse, fallbackGroups: OrgGroup[]) {
		groups = (response.availableGroups ?? fallbackGroups).map((group) => ({ ...group }));
		if (!response.records) return;
		userRecords = response.records.map(normalizeOrgProfileRecord);
		originalProfiles = Object.fromEntries(userRecords.map((record) => [record.userID, orgProfileSnapshot(record)]));
		editingRecordsByUserID = reconcileEditingRecords(editingRecordsByUserID, userRecords, groups);
	}

	async function loadUsers() {
		if (!fleetID || !adminBaseURL) return;
		isLoading = true;
		errorMessage = '';
		try {
			applyUsersResponse(await fetchUsers(adminBaseURL, text.messages.usersLoadError));
		} catch (error) {
			errorMessage = apiErrorMessage(error, text.messages.usersLoadError);
		} finally {
			isLoading = false;
		}
	}

	async function persistGroups(nextGroups: OrgGroup[]) {
		isSavingGroups = true;
		errorMessage = '';
		try {
			const response = await saveOrgGroups(adminBaseURL, nextGroups, text.messages.userSaveError);
			applyGroupsResponse(response, nextGroups);
			return true;
		} catch (error) {
			errorMessage = apiErrorMessage(error, text.messages.userSaveError);
			return false;
		} finally {
			isSavingGroups = false;
		}
	}

	async function addGroup(name: string): Promise<string> {
		const trimmedName = name.trim();
		if (!trimmedName) return '';
		const existingGroupID = groupIDByName(trimmedName);
		if (existingGroupID) return existingGroupID;
		return createGroup(trimmedName);
	}

	async function addGlobalGroup() {
		const groupID = await addGroup(newGroupName);
		if (!groupID) return;
		newGroupName = '';
	}

	async function createGroup(name: string) {
		const group = { id: crypto.randomUUID(), name };
		const isPersisted = await persistGroups([...groups, group]);
		return isPersisted ? group.id : '';
	}

	function groupIDByName(name: string) {
		const normalizedName = normalizedGroupName(name);
		return groups.find((group) => normalizedGroupName(group.name) === normalizedName)?.id ?? '';
	}

	function normalizedGroupName(name: string) {
		return name.trim().toLowerCase();
	}

	function editProfile(record: UserRecord) {
		if (editingRecordsByUserID[record.userID]) return;
		errorMessage = '';
		editingRecordsByUserID = {
			...editingRecordsByUserID,
			[record.userID]: copyUserRecord(record)
		};
	}

	function cancelProfileEdit(userID: string) {
		errorMessage = '';
		removeEditingRecord(userID);
	}

	function removeEditingRecord(userID: string) {
		editingRecordsByUserID = Object.fromEntries(Object.entries(editingRecordsByUserID).filter(([candidateUserID]) => candidateUserID !== userID));
	}

	async function saveProfile(userID: string) {
		const record = editingRecordsByUserID[userID];
		if (!record) return;
		if (!fleetID || !adminBaseURL || isSavingProfile(userID) || hasInvalidSupervisor(record)) return;
		if (!isChanged(record)) {
			errorMessage = '';
			removeEditingRecord(userID);
			return;
		}
		savingProfileUserIDs = { ...savingProfileUserIDs, [userID]: true };
		errorMessage = '';
		try {
			const profiles = [orgProfileUpdate(record)];
			applyUsersResponse(await saveOrgProfiles(adminBaseURL, profiles, text.messages.userSaveError));
			errorMessage = '';
			removeEditingRecord(userID);
		} catch (error) {
			errorMessage = apiErrorMessage(error, text.messages.userSaveError);
		} finally {
			savingProfileUserIDs = Object.fromEntries(Object.entries(savingProfileUserIDs).filter(([candidateUserID]) => candidateUserID !== userID));
		}
	}
</script>

<div class="flex flex-wrap items-center justify-between gap-3">
	<div>
		<h2 class="text-base font-semibold">{text.orgchart.title}</h2>
		<p class="text-muted-foreground text-sm">{text.orgchart.description}</p>
	</div>
	{#if isDeviceContext && userRecords.length > 0}
		<div class="flex items-center gap-3">
			<label class="flex items-center gap-2 text-sm">
				<Switch bind:checked={isEditing} aria-label={text.orgchart.editMode} />
				{text.orgchart.editMode}
			</label>
		</div>
	{/if}
</div>

{#if !isDeviceContext}
	<p class="text-muted-foreground rounded-md border bg-muted/30 px-3 py-2 text-sm">{text.users.deviceOnly}</p>
{:else if errorMessage}
	<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{errorMessage}</p>
{/if}

{#if isDeviceContext}
	{#if isLoading}
		<p class="text-muted-foreground text-sm">{text.users.loading}</p>
	{:else if userRecords.length === 0}
		<p class="text-muted-foreground text-sm">{text.users.empty}</p>
	{:else}
		{#if isEditing}
			<form class="grid gap-1.5 rounded-lg border bg-card p-3 shadow-sm" onsubmit={(event) => { event.preventDefault(); addGlobalGroup(); }}>
				<Label class="text-xs" for="new-orgchart-group">{text.orgchart.newGroup}</Label>
				<div class="flex gap-2">
					<Input id="new-orgchart-group" class="h-8 text-sm" bind:value={newGroupName} placeholder={text.orgchart.groupPlaceholder} autocomplete="off" disabled={isSavingGroups} />
					<Button type="submit" size="sm" disabled={isSavingGroups || !newGroupName.trim()} class="h-8 gap-2">
						<PlusIcon class="size-4" />
						{text.orgchart.addOrganization}
					</Button>
				</div>
			</form>
		{/if}
		<div class="grid gap-2">
			{#each orgForest(userRecords) as node (node.record.email)}
				{@render orgNode(node)}
			{/each}
		</div>
	{/if}
{/if}

{#snippet orgNode(node: OrgNode)}
	{@const editingRecord = editingRecordsByUserID[node.record.userID]}
	<div class="grid min-w-0 gap-2">
		{@render orgProfile(node.record, editingRecord)}
		{#if node.reports.length > 0}
			<div class="ml-5 grid gap-2 border-l pl-4" data-testid={`orgchart-reports-${node.record.userID}`}>
				{#each node.reports as report (report.record.email)}
					{@render orgNode(report)}
				{/each}
			</div>
		{/if}
	</div>
{/snippet}

{#snippet orgProfile(record: UserRecord, editingRecord: UserRecord | undefined)}
	{#if editingRecord}
		<OrgchartProfileCard
			record={editingRecord}
			{userRecords}
			{groups}
			{text}
			canEdit={isEditing}
			isEditing={true}
			isSaving={isSavingProfile(editingRecord.userID)}
			hasInvalidSupervisor={hasInvalidSupervisor(editingRecord)}
			onEdit={() => editProfile(record)}
			onSave={() => saveProfile(record.userID)}
			onCancel={() => cancelProfileEdit(record.userID)}
		/>
	{:else}
		<OrgchartProfileCard
			{record}
			{userRecords}
			{groups}
			{text}
			canEdit={isEditing}
			isEditing={false}
			isSaving={isSavingProfile(record.userID)}
			hasInvalidSupervisor={hasInvalidSupervisor(record)}
			onEdit={() => editProfile(record)}
			onSave={() => saveProfile(record.userID)}
			onCancel={() => cancelProfileEdit(record.userID)}
		/>
	{/if}
{/snippet}
