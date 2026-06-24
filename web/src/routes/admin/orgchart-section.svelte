<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { Switch } from '$lib/components/ui/switch';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { apiErrorMessage, fetchUsers, saveUser } from './admin-api';
	import type { AdminPageText, CircleRecord, UserRecord, UsersResponse } from './admin-types';

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

	let { adminBaseURL, fleetID, isDeviceContext, text }: OrgchartSectionProps = $props();

	let loadedAdminBaseURL = $state('');
	let userRecords = $state<UserRecord[]>([]);
	let availableCircles = $state<CircleRecord[]>([]);
	let isEditing = $state(false);
	let isLoading = $state(false);
	let savingUserID = $state('');
	let errorMessage = $state('');

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		loadUsers();
	});

	function applyUsersResponse(response: UsersResponse) {
		availableCircles = response.availableCircles ?? [];
		userRecords = (response.records ?? []).map((record) => ({
			...record,
			name: record.name ?? '',
			jobTitle: record.jobTitle ?? '',
			primaryCircle: record.primaryCircle ?? '',
			supervisorID: record.supervisorID ?? ''
		}));
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

	function circleName(circleID: string) {
		return availableCircles.find((circle) => circle.circleID === circleID)?.displayName || circleID;
	}

	function memberCircles(record: UserRecord) {
		return (record.circles ?? []).filter((circle) => circle !== 'admin');
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

	async function saveMember(record: UserRecord) {
		if (!fleetID || !adminBaseURL) return;
		savingUserID = record.userID;
		errorMessage = '';
		try {
			applyUsersResponse(await saveUser(
				adminBaseURL,
				{
					userID: record.userID,
					handle: record.handle,
					name: record.name?.trim() ?? '',
					hireDate: record.hireDate ?? '',
					email: record.email,
					role: record.role,
					circles: record.circles ?? ['staff'],
					mattermostUserID: record.mattermostUserID,
					mattermostUsername: record.mattermostUsername,
					status: record.status,
					jobTitle: record.jobTitle?.trim() ?? '',
					primaryCircle: record.primaryCircle ?? '',
					supervisorID: record.supervisorID ?? ''
				},
				text.messages.userSaveError
			));
		} catch (error) {
			errorMessage = apiErrorMessage(error, text.messages.userSaveError);
		} finally {
			savingUserID = '';
		}
	}
</script>

<div class="flex flex-wrap items-center justify-between gap-3">
	<div>
		<h2 class="text-base font-semibold">{text.orgchart.title}</h2>
		<p class="text-muted-foreground text-sm">{text.orgchart.description}</p>
	</div>
	{#if isDeviceContext && userRecords.length > 0}
		<label class="flex items-center gap-2 text-sm">
			<Switch bind:checked={isEditing} />
			{text.orgchart.editMode}
		</label>
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
				{#if record.primaryCircle}
					<Badge variant="secondary" class="shrink-0">{circleName(record.primaryCircle)}</Badge>
				{/if}
			</div>

			{#if isEditing}
				<div class="mt-3 grid gap-2 border-t pt-3 sm:grid-cols-[1fr_1fr_1fr_auto] sm:items-end">
					<label class="grid gap-1.5">
						<Label class="text-xs">{text.orgchart.jobTitle}</Label>
						<Input bind:value={record.jobTitle} placeholder={text.orgchart.jobTitlePlaceholder} autocomplete="off" />
					</label>
					<label class="grid gap-1.5">
						<Label class="text-xs">{text.orgchart.primaryCircle}</Label>
						<Select.Root type="single" bind:value={record.primaryCircle}>
							<Select.Trigger class="w-full">
								{record.primaryCircle ? circleName(record.primaryCircle) : text.orgchart.none}
							</Select.Trigger>
							<Select.Content>
								<Select.Item value="" label={text.orgchart.none}>{text.orgchart.none}</Select.Item>
								{#each memberCircles(record) as circleID (circleID)}
									<Select.Item value={circleID} label={circleName(circleID)}>{circleName(circleID)}</Select.Item>
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
					<Button variant="outline" size="sm" disabled={savingUserID === record.userID} onclick={() => saveMember(record)} class="gap-2">
						{#if savingUserID === record.userID}
							<RefreshCwIcon class="size-4 animate-spin" />
						{/if}
						{text.users.save}
					</Button>
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
