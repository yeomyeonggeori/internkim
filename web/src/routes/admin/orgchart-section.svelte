<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { Switch } from '$lib/components/ui/switch';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import XIcon from '@lucide/svelte/icons/x';
	import { apiErrorMessage, fetchUsers, saveOrgGroups, saveOrgProfiles } from './admin-api';
	import type { AdminPageText, OrgGroup, UserRecord, UsersResponse } from './admin-types';

	type OrgchartSectionProps = {
		adminBaseURL: string;
		fleetID: string;
		isDeviceContext: boolean;
		text: AdminPageText;
	};

	type OrgNode = {
		record: UserRecord;
		reports: OrgNode[];
	};

	type OrgProfile = {
		jobTitle: string;
		group: string;
		supervisorID: string;
	};

	let { adminBaseURL, fleetID, isDeviceContext, text }: OrgchartSectionProps = $props();

	let loadedAdminBaseURL = $state('');
	let userRecords = $state<UserRecord[]>([]);
	let groups = $state<OrgGroup[]>([]);
	let originalProfiles = $state<Record<string, OrgProfile>>({});
	let newGroupName = $state('');
	let isEditing = $state(false);
	let isLoading = $state(false);
	let isSaving = $state(false);
	let errorMessage = $state('');

	function profileOf(record: UserRecord): OrgProfile {
		return {
			jobTitle: record.jobTitle?.trim() ?? '',
			group: record.group ?? '',
			supervisorID: record.supervisorID ?? ''
		};
	}

	function isChanged(record: UserRecord) {
		const original = originalProfiles[record.userID];
		if (!original) return false;
		const current = profileOf(record);
		return current.jobTitle !== original.jobTitle || current.group !== original.group || current.supervisorID !== original.supervisorID;
	}

	function changedRecords() {
		return userRecords.filter(isChanged);
	}

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		loadUsers();
	});

	function applyUsersResponse(response: UsersResponse) {
		groups = (response.availableGroups ?? []).map((group) => ({ ...group }));
		userRecords = (response.records ?? []).map((record) => ({
			...record,
			name: record.name ?? '',
			jobTitle: record.jobTitle ?? '',
			group: record.group ?? '',
			supervisorID: record.supervisorID ?? ''
		}));
		originalProfiles = Object.fromEntries(userRecords.map((record) => [record.userID, profileOf(record)]));
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

	function groupName(groupID: string) {
		return groups.find((group) => group.id === groupID)?.name ?? '';
	}

	function personLabel(record: UserRecord) {
		return record.name || record.email;
	}

	function supervisorLabel(supervisorID: string) {
		const supervisor = userRecords.find((candidate) => candidate.userID === supervisorID);
		return supervisor ? personLabel(supervisor) : text.orgchart.none;
	}

	function byName(first: UserRecord, second: UserRecord) {
		return personLabel(first).localeCompare(personLabel(second));
	}

	function orgForest(): OrgNode[] {
		const reportsBySupervisor = new Map<string, UserRecord[]>();
		const knownID = new Set(userRecords.map((record) => record.userID));
		for (const record of userRecords) {
			const supervisorID = record.supervisorID?.trim() ?? '';
			const key = supervisorID && knownID.has(supervisorID) ? supervisorID : '';
			if (!reportsBySupervisor.has(key)) reportsBySupervisor.set(key, []);
			reportsBySupervisor.get(key)?.push(record);
		}
		const placed = new Set<string>();
		const buildNode = (record: UserRecord): OrgNode => {
			placed.add(record.userID);
			const reports = (reportsBySupervisor.get(record.userID) ?? [])
				.filter((report) => !placed.has(report.userID))
				.sort(byName)
				.map(buildNode);
			return { record, reports };
		};
		const roots = (reportsBySupervisor.get('') ?? []).sort(byName).map(buildNode);
		for (const record of userRecords) {
			if (!placed.has(record.userID)) roots.push(buildNode(record));
		}
		return roots;
	}

	async function persistGroups(nextGroups: OrgGroup[]) {
		isSaving = true;
		errorMessage = '';
		try {
			applyUsersResponse(await saveOrgGroups(adminBaseURL, nextGroups, text.messages.userSaveError));
		} catch (error) {
			errorMessage = apiErrorMessage(error, text.messages.userSaveError);
		} finally {
			isSaving = false;
		}
	}

	function addGroup() {
		const name = newGroupName.trim();
		if (!name) return;
		newGroupName = '';
		persistGroups([...groups, { id: crypto.randomUUID(), name }]);
	}

	function removeGroup(groupID: string) {
		persistGroups(groups.filter((group) => group.id !== groupID));
	}

	function renameGroup() {
		const named = groups.filter((group) => group.name.trim());
		persistGroups(named.map((group) => ({ id: group.id, name: group.name.trim() })));
	}

	async function saveProfiles() {
		if (!fleetID || !adminBaseURL) return;
		const pending = changedRecords();
		if (pending.length === 0) return;
		isSaving = true;
		errorMessage = '';
		try {
			const profiles = pending.map((record) => ({
				userID: record.userID,
				email: record.email,
				jobTitle: record.jobTitle?.trim() ?? '',
				group: record.group ?? '',
				supervisorID: record.supervisorID ?? ''
			}));
			applyUsersResponse(await saveOrgProfiles(adminBaseURL, profiles, text.messages.userSaveError));
		} catch (error) {
			errorMessage = apiErrorMessage(error, text.messages.userSaveError);
		} finally {
			isSaving = false;
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
			{#if isEditing}
				<Button size="sm" disabled={isSaving || changedRecords().length === 0} onclick={saveProfiles} class="gap-2">
					{#if isSaving}
						<RefreshCwIcon class="size-4 animate-spin" />
					{/if}
					{text.users.save}{#if changedRecords().length > 0}&nbsp;{changedRecords().length}{/if}
				</Button>
			{/if}
			<label class="flex items-center gap-2 text-sm">
				<Switch bind:checked={isEditing} />
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
	{#if isEditing}
		<Card.Root>
			<Card.Header class="gap-1">
				<Card.Title class="text-sm">{text.orgchart.manageGroups}</Card.Title>
			</Card.Header>
			<Card.Content class="grid gap-3">
				{#if groups.length > 0}
					<div class="grid gap-2 sm:grid-cols-2">
						{#each groups as group (group.id)}
							<div class="flex items-center gap-2">
								<Input bind:value={group.name} onchange={renameGroup} autocomplete="off" />
								<Button variant="ghost" size="icon-sm" disabled={isSaving} onclick={() => removeGroup(group.id)} aria-label={text.users.remove}>
									<XIcon class="size-4" />
								</Button>
							</div>
						{/each}
					</div>
				{/if}
				<form class="flex items-center gap-2" onsubmit={(event) => { event.preventDefault(); addGroup(); }}>
					<Input bind:value={newGroupName} placeholder={text.orgchart.groupPlaceholder} autocomplete="off" />
					<Button type="submit" size="sm" disabled={isSaving || !newGroupName.trim()} class="gap-2">
						<PlusIcon class="size-4" />
						{text.orgchart.addGroup}
					</Button>
				</form>
			</Card.Content>
		</Card.Root>
	{/if}

	{#if isLoading}
		<p class="text-muted-foreground text-sm">{text.users.loading}</p>
	{:else if userRecords.length === 0}
		<p class="text-muted-foreground text-sm">{text.users.empty}</p>
	{:else}
		<div class="grid gap-2.5">
			{#each orgForest() as node (node.record.email)}
				{@render orgNode(node)}
			{/each}
		</div>
	{/if}
{/if}

{#snippet orgNode(node: OrgNode)}
	{@const record = node.record}
	<div class="grid gap-2.5">
		<div class="rounded-xl border bg-card px-3.5 py-3 shadow-sm">
			<div class="flex min-w-0 items-center gap-3">
				<PersonAvatar name={record.name} email={record.email} class="size-9" />
				<div class="flex min-w-0 flex-1 flex-wrap items-baseline gap-x-2 gap-y-0.5">
					<span class="truncate text-sm font-medium">{personLabel(record)}</span>
					{#if record.jobTitle}
						<span class="text-muted-foreground truncate text-xs">{record.jobTitle}</span>
					{/if}
				</div>
				{#if record.group && groupName(record.group)}
					<Badge variant="secondary" class="shrink-0">{groupName(record.group)}</Badge>
				{/if}
			</div>

			{#if isEditing}
				<div class="mt-3 grid gap-2 border-t pt-3 sm:grid-cols-3">
					<label class="grid gap-1.5">
						<Label class="text-xs">{text.orgchart.jobTitle}</Label>
						<Input bind:value={record.jobTitle} placeholder={text.orgchart.jobTitlePlaceholder} autocomplete="off" />
					</label>
					<label class="grid gap-1.5">
						<Label class="text-xs">{text.orgchart.group}</Label>
						<Select.Root type="single" bind:value={record.group}>
							<Select.Trigger class="w-full">
								{record.group ? groupName(record.group) || text.orgchart.none : text.orgchart.none}
							</Select.Trigger>
							<Select.Content>
								<Select.Item value="" label={text.orgchart.none}>{text.orgchart.none}</Select.Item>
								{#each groups as group (group.id)}
									<Select.Item value={group.id} label={group.name}>{group.name}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</label>
					<label class="grid gap-1.5">
						<Label class="text-xs">{text.orgchart.supervisor}</Label>
						<Select.Root type="single" bind:value={record.supervisorID}>
							<Select.Trigger class="w-full">
								{record.supervisorID ? supervisorLabel(record.supervisorID) : text.orgchart.none}
							</Select.Trigger>
							<Select.Content>
								<Select.Item value="" label={text.orgchart.none}>{text.orgchart.none}</Select.Item>
								{#each userRecords.filter((candidate) => candidate.userID !== record.userID) as option (option.userID)}
									<Select.Item value={option.userID} label={personLabel(option)}>{personLabel(option)}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</label>
				</div>
			{/if}
		</div>

		{#if node.reports.length > 0}
			<div class="ml-4 grid gap-2.5 border-l pl-4 sm:ml-5 sm:pl-5">
				{#each node.reports as report (report.record.email)}
					{@render orgNode(report)}
				{/each}
			</div>
		{/if}
	</div>
{/snippet}
