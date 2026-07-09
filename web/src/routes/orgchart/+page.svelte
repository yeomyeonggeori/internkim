<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Sheet from '$lib/components/ui/sheet';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { onMount } from 'svelte';
	import { adminText } from '../admin/text';
	import OrgchartAddOrganizationPopover from './orgchart-add-organization-popover.svelte';
	import { OrgchartDirectoryController } from './orgchart-directory-controller.svelte';
	import OrgchartFilterPopover from './orgchart-filter-popover.svelte';
	import OrgchartOrganizationCard from './orgchart-organization-card.svelte';
	import OrgchartPersonDetailPanel from './orgchart-person-detail-panel.svelte';
	import { orgchartDirectoryText } from './text';

	const text = createPageText(orgchartDirectoryText);
	const adminPageText = createPageText(adminText);
	const adminBaseURL = '/admin/api';
	const detailSheetViewport = new IsMobile(1024);
	const controller = new OrgchartDirectoryController(adminBaseURL, text, adminPageText);
	const isDetailSheetOpen = $derived(detailSheetViewport.current && Boolean(controller.selectedRecord));

	onMount(() => {
		void controller.load();
	});

	function handleDetailSheetOpenChange(nextOpen: boolean): void {
		if (nextOpen || !detailSheetViewport.current) return;
		controller.clearSelection();
	}
</script>

<svelte:head>
	<title>{text.title}</title>
</svelte:head>

<main class="min-h-full w-full flex-1 bg-background text-foreground">
	<div class="grid h-[calc(100vh-3rem)] min-h-0 grid-rows-[auto_minmax(0,1fr)]">
		<header class="border-b bg-background px-4 py-3 sm:px-6 sm:py-4">
			<div class="grid gap-3 lg:flex lg:flex-wrap lg:items-center lg:justify-between lg:gap-4">
				<div class="flex min-h-10 items-center justify-between gap-3">
					<h1 class="text-xl font-semibold">{text.title}</h1>
					{#if detailSheetViewport.current}
					<div class="flex shrink-0 items-center gap-2">
						<OrgchartFilterPopover
							bind:isOpen={controller.isFilterOpen}
							selectedGroupID={controller.groupID}
							options={controller.options}
							{text}
							buttonSize="sm"
							onSelectGroup={(groupID) => controller.selectGroup(groupID)}
						/>
						{#if controller.canManage}
							<OrgchartAddOrganizationPopover
								bind:isOpen={controller.isAddingGroup}
								bind:newGroupName={controller.newGroupName}
								isSaving={controller.isSavingGroups}
								text={adminPageText.orgchart}
								inputID="new-orgchart-group-mobile"
								buttonSize="sm"
								onAdd={() => controller.addGlobalGroup()}
								onCancel={() => controller.cancelAddGroup()}
								onOpenChange={(nextOpen) => controller.handleAddGroupOpenChange(nextOpen)}
							/>
						{/if}
					</div>
					{/if}
				</div>

				<div class="flex w-full flex-wrap items-center justify-end gap-2 lg:w-auto">
					<div class="relative w-full sm:w-72">
						<SearchIcon class="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
						<Input id="orgchart-search" class="pl-9" bind:value={controller.query} placeholder={text.searchPlaceholder} autocomplete="off" aria-label={text.search} />
					</div>
					{#if !detailSheetViewport.current}
						<OrgchartFilterPopover
							bind:isOpen={controller.isFilterOpen}
							selectedGroupID={controller.groupID}
							options={controller.options}
							{text}
							onSelectGroup={(groupID) => controller.selectGroup(groupID)}
						/>
						{#if controller.canManage}
							<OrgchartAddOrganizationPopover
								bind:isOpen={controller.isAddingGroup}
								bind:newGroupName={controller.newGroupName}
								isSaving={controller.isSavingGroups}
								text={adminPageText.orgchart}
								inputID="new-orgchart-group"
								onAdd={() => controller.addGlobalGroup()}
								onCancel={() => controller.cancelAddGroup()}
								onOpenChange={(nextOpen) => controller.handleAddGroupOpenChange(nextOpen)}
							/>
						{/if}
					{/if}
				</div>
			</div>
		</header>

		<section class="h-full min-h-0 min-w-0 overflow-auto px-4 py-5 sm:px-6 lg:overflow-hidden">
			<div class="grid h-full min-h-0 grid-rows-[auto_minmax(0,1fr)] gap-3">
				{#if controller.errorMessage}
					<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{controller.errorMessage}</p>
				{:else}
					<div class="hidden"></div>
				{/if}

				{#if controller.isLoading}
					<p class="text-sm text-muted-foreground">{text.loading}</p>
				{:else}
					<div class={['grid h-full min-h-0 gap-4 lg:overflow-hidden', controller.selectedRecord ? 'lg:grid-cols-[minmax(0,1fr)_320px] 2xl:grid-cols-[minmax(0,1fr)_360px]' : '']} data-testid="orgchart-board">
						<div class="min-h-0 lg:h-full lg:overflow-hidden">
							<div class="min-h-0 pb-6 pr-1 lg:h-full lg:overflow-y-auto" data-testid="orgchart-list-scroll">
								{#if controller.visibleRecords.length === 0}
									<p class="rounded-md border bg-muted/30 px-4 py-5 text-sm text-muted-foreground">{text.empty}</p>
								{:else}
									<div class={['grid gap-4', controller.selectedRecord ? 'xl:grid-cols-2' : 'lg:grid-cols-2']} data-testid="orgchart-organization-grid">
										{#each controller.organizationSections as section (section.id)}
											<OrgchartOrganizationCard
												{section}
												selectedUserID={controller.selectedUserID}
												{text}
												selectRecord={(record) => controller.selectRecord(record)}
											/>
										{/each}
									</div>
								{/if}
							</div>
						</div>

						{#if controller.selectedRecord && !detailSheetViewport.current}
							<div class="min-h-0" data-testid="orgchart-detail-column">
								<OrgchartPersonDetailPanel
									record={controller.selectedRecord}
									groups={controller.groups}
									{text}
									adminText={adminPageText}
									canEdit={controller.canManage}
									isEditing={controller.selectedRecordIsEditing}
									editingRecord={controller.selectedEditingRecord}
									userRecords={controller.records}
									isSaving={controller.isSavingSelectedProfile}
									hasInvalidSupervisor={controller.hasInvalidSelectedSupervisor}
									onEdit={() => controller.selectedRecord && controller.editRecord(controller.selectedRecord)}
									onSave={() => controller.selectedRecord && controller.saveProfile(controller.selectedRecord.userID)}
									onCancel={() => controller.selectedRecord && controller.cancelProfileEdit(controller.selectedRecord.userID)}
									clearSelection={() => controller.clearSelection()}
								/>
							</div>
						{/if}
					</div>
				{/if}
			</div>
		</section>
	</div>

	{#if controller.selectedRecord && detailSheetViewport.current}
		<Sheet.Root open={isDetailSheetOpen} onOpenChange={handleDetailSheetOpenChange}>
			<Sheet.Content
				side="bottom"
				class="max-h-[85svh] overflow-hidden rounded-t-xl p-0 lg:hidden"
				showCloseButton={false}
				closeLabel={text.closeDetail}
				data-testid="orgchart-mobile-detail-sheet"
			>
				<Sheet.Header class="sr-only">
					<Sheet.Title>{text.personDetail}</Sheet.Title>
					<Sheet.Description>{controller.selectedRecord.name || controller.selectedRecord.email}</Sheet.Description>
				</Sheet.Header>
				<OrgchartPersonDetailPanel
					record={controller.selectedRecord}
					groups={controller.groups}
					{text}
					adminText={adminPageText}
					canEdit={controller.canManage}
					isEditing={controller.selectedRecordIsEditing}
					editingRecord={controller.selectedEditingRecord}
					userRecords={controller.records}
					isSaving={controller.isSavingSelectedProfile}
					hasInvalidSupervisor={controller.hasInvalidSelectedSupervisor}
					variant="sheet"
					onEdit={() => controller.selectedRecord && controller.editRecord(controller.selectedRecord)}
					onSave={() => controller.selectedRecord && controller.saveProfile(controller.selectedRecord.userID)}
					onCancel={() => controller.selectedRecord && controller.cancelProfileEdit(controller.selectedRecord.userID)}
					clearSelection={() => controller.clearSelection()}
				/>
			</Sheet.Content>
		</Sheet.Root>
	{/if}
</main>
