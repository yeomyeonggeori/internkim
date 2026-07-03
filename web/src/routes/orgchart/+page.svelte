<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import FilterIcon from '@lucide/svelte/icons/filter';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { onMount } from 'svelte';
	import type { OrgGroup, UserRecord } from '../admin/admin-types';
	import { fetchAdminSession } from '../admin/admin-api';
	import { fetchOrgchartDirectory, orgchartApiErrorMessage } from './orgchart-api';
	import {
		filterOrgchartRecords,
		orgchartCanvasModel,
		orgchartFilterOptions,
		unassignedGroupID
	} from './orgchart-directory-model';
	import OrgchartOrgCanvas from './orgchart-org-canvas.svelte';
	import OrgchartPersonDetailPanel from './orgchart-person-detail-panel.svelte';
	import { orgchartDirectoryText } from './text';

	type OrgchartTab = 'chart' | 'list';
	type PopupAnchor = {
		top: number;
		right: number;
		bottom: number;
		left: number;
		width: number;
		height: number;
	};

	const text = createPageText(orgchartDirectoryText);
	const allValue = '__all__';

	let records = $state<UserRecord[]>([]);
	let groups = $state<OrgGroup[]>([]);
	let query = $state('');
	let groupID = $state('');
	let selectedUserID = $state('');
	let selectedAnchor = $state<PopupAnchor | undefined>();
	let activeTab = $state<OrgchartTab>('chart');
	let isFilterOpen = $state(false);
	let zoom = $state(100);
	let isLoading = $state(true);
	let canManage = $state(false);
	let errorMessage = $state('');

	const filters = $derived({ query, groupID });
	const options = $derived(orgchartFilterOptions(records, groups));
	const visibleRecords = $derived(filterOrgchartRecords(records, filters));
	const canvasModel = $derived(orgchartCanvasModel(visibleRecords, groups, text.unassignedTeam));
	const selectedRecord = $derived(visibleRecords.find((record) => record.userID === selectedUserID));

	onMount(() => {
		loadDirectory();
		loadAdminAccess();
	});

	async function loadDirectory() {
		isLoading = true;
		errorMessage = '';
		try {
			const response = await fetchOrgchartDirectory(text.loadError);
			const nextRecords = response.records ?? [];
			const nextGroups = response.availableGroups ?? [];
			records = nextRecords;
			groups = nextGroups;
			clearSelection();
		} catch (error) {
			errorMessage = orgchartApiErrorMessage(error, text.loadError);
		} finally {
			isLoading = false;
		}
	}

	async function loadAdminAccess() {
		try {
			const session = await fetchAdminSession('/admin/api', '');
			canManage = session.isAdmin === true;
		} catch {
			canManage = false;
		}
	}

	function setActiveTab(tab: OrgchartTab): void {
		activeTab = tab;
		clearSelection();
	}

	function selectGroup(value: string | undefined): void {
		groupID = value === allValue || value === undefined ? '' : value;
		clearSelection();
	}

	function selectRecord(record: UserRecord, anchor: PopupAnchor): void {
		selectedUserID = record.userID;
		selectedAnchor = anchor;
	}

	function clearSelection(): void {
		selectedUserID = '';
		selectedAnchor = undefined;
	}

	function groupName(record: UserRecord): string {
		const normalizedGroupID = (record.primaryGroupID ?? record.group ?? '').trim();
		if (!normalizedGroupID) return text.unassignedTeam;
		return groups.find((group) => group.id === normalizedGroupID)?.name ?? normalizedGroupID;
	}

</script>

<svelte:head>
	<title>{text.title}</title>
</svelte:head>

<main class="min-h-full w-full flex-1 bg-background text-foreground">
	<div class="grid min-h-[calc(100vh-3rem)] grid-rows-[auto_minmax(0,1fr)]">
		<header class="border-b bg-background px-4 py-4 sm:px-6">
			<div class="flex flex-wrap items-center justify-between gap-4">
				<div class="flex h-10 items-end gap-6">
					<button
						type="button"
						class={['h-10 border-b-2 text-sm font-semibold', activeTab === 'chart' ? 'border-primary text-primary' : 'border-transparent text-muted-foreground']}
						onclick={() => setActiveTab('chart')}
					>
						{text.orgchartTab}
					</button>
					<button
						type="button"
						class={['h-10 border-b-2 text-sm font-semibold', activeTab === 'list' ? 'border-primary text-primary' : 'border-transparent text-muted-foreground']}
						onclick={() => setActiveTab('list')}
					>
						{text.listTab}
					</button>
				</div>

				<div class="flex w-full flex-wrap items-center justify-end gap-2 sm:w-auto">
					<div class="relative w-full sm:w-72">
						<SearchIcon class="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
						<Input id="orgchart-search" class="pl-9" bind:value={query} placeholder={text.searchPlaceholder} autocomplete="off" aria-label={text.search} />
					</div>
					<Button variant="outline" onclick={() => (isFilterOpen = !isFilterOpen)} aria-pressed={isFilterOpen}>
						<FilterIcon class="size-4" />
						{text.filter}
					</Button>
					{#if canManage}
						<Button href="/admin/?section=orgchart">{text.manageInAdmin}</Button>
					{/if}
				</div>
			</div>

			{#if isFilterOpen}
				<section class="mt-4 grid gap-3 rounded-lg border bg-card p-3 shadow-sm md:max-w-80">
					<div class="grid gap-1.5">
						<Label>{text.organization}</Label>
						<Select.Root type="single" value={groupID || allValue} onValueChange={selectGroup}>
							<Select.Trigger class="w-full" aria-label={text.organization}>
								{groupID === unassignedGroupID ? text.unassignedTeam : groupID ? options.groups.find((group) => group.id === groupID)?.name : text.allOrganizations}
							</Select.Trigger>
							<Select.Content>
								<Select.Item value={allValue} label={text.allOrganizations}>{text.allOrganizations}</Select.Item>
								{#each options.groups as group}
									<Select.Item value={group.id} label={group.name}>{group.name}</Select.Item>
								{/each}
								{#if options.hasUnassigned}
									<Select.Item value={unassignedGroupID} label={text.unassignedTeam}>{text.unassignedTeam}</Select.Item>
								{/if}
							</Select.Content>
						</Select.Root>
					</div>
				</section>
			{/if}
		</header>

		<div class="grid min-h-0">
			<section class="min-h-0 min-w-0 overflow-auto px-4 py-5 sm:px-6">
				{#if errorMessage}
					<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{errorMessage}</p>
				{:else if isLoading}
					<p class="text-sm text-muted-foreground">{text.loading}</p>
				{:else if visibleRecords.length === 0}
					<p class="rounded-md border bg-muted/30 px-4 py-5 text-sm text-muted-foreground">{text.empty}</p>
				{:else if activeTab === 'chart'}
					<OrgchartOrgCanvas model={canvasModel} {selectedUserID} {zoom} {text} {selectRecord} setZoom={(nextZoom) => (zoom = nextZoom)} />
				{:else}
					<div class="grid gap-2 rounded-lg border bg-card p-3 shadow-sm" data-testid="orgchart-people-list">
						{#each visibleRecords as record (record.userID)}
							<button
								type="button"
								class={['grid gap-3 rounded-md border p-3 text-left hover:bg-muted/40', selectedUserID === record.userID ? 'border-primary ring-1 ring-primary' : 'border-border']}
								onclick={(event) => {
									event.stopPropagation();
									selectRecord(record, event.currentTarget.getBoundingClientRect());
								}}
								data-testid={`orgchart-list-row-${record.userID}`}
							>
								<span class="min-w-0">
									<span class="block truncate text-sm font-semibold">{record.name || record.email}</span>
									<span class="block truncate text-xs text-muted-foreground">{record.jobTitle || text.noTitle} · {groupName(record)}</span>
								</span>
							</button>
						{/each}
					</div>
				{/if}
			</section>

			<OrgchartPersonDetailPanel record={selectedRecord} anchor={selectedAnchor} {groups} {text} {clearSelection} />
		</div>
	</div>
</main>
