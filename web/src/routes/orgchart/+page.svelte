<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Sheet from '$lib/components/ui/sheet';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import PanelLeftIcon from '@lucide/svelte/icons/panel-left';
	import SearchIcon from '@lucide/svelte/icons/search';
	import { onMount } from 'svelte';
	import { adminText } from '../admin/text';
	import OrgchartAddOrganizationPopover from './orgchart-add-organization-popover.svelte';
	import { OrgchartDirectoryController } from './orgchart-directory-controller.svelte';
	import OrgchartFilterPopover from './orgchart-filter-popover.svelte';
	import OrgchartOrganizationTree from './orgchart-organization-tree.svelte';
	import OrgchartPeopleLayer from './orgchart-people-layer.svelte';
	import OrgchartPersonDetailPanel from './orgchart-person-detail-panel.svelte';
	import { orgchartDirectoryText } from './text';

	const text = createPageText(orgchartDirectoryText);
	const adminPageText = createPageText(adminText);
	const adminBaseURL = '/admin/api';
	const detailSheetViewport = new IsMobile(1024);
	const controller = new OrgchartDirectoryController(adminBaseURL, text, adminPageText);
	const isDetailSheetOpen = $derived(detailSheetViewport.current && Boolean(controller.selectedRecord));
	let isOrganizationSheetOpen = $state(false);

	onMount(() => {
		void controller.load();
	});

	function handleDetailSheetOpenChange(nextOpen: boolean): void {
		if (nextOpen || !detailSheetViewport.current) return;
		controller.clearSelection();
	}

	function selectOrganization(organizationID: string): void {
		controller.selectGroup(organizationID);
		if (detailSheetViewport.current) isOrganizationSheetOpen = false;
	}
</script>

<svelte:head>
	<title>{text.title}</title>
</svelte:head>

<main class="h-full min-h-0 w-full flex-1 overflow-hidden bg-background text-foreground">
	<div class="grid h-[calc(100vh-3rem)] min-h-0 grid-rows-[auto_minmax(0,1fr)]">
		<header class="border-b bg-background px-4 py-3 sm:px-6">
			<div class="grid gap-3 lg:flex lg:items-center lg:justify-between">
				<div class="flex min-h-10 items-center justify-between gap-3">
					<h1 class="text-xl font-semibold">{text.title}</h1>
					{#if detailSheetViewport.current}
						<div class="flex shrink-0 items-center gap-2">
							<Button type="button" size="sm" variant="outline" onclick={() => (isOrganizationSheetOpen = true)}>
								<PanelLeftIcon class="size-4" />
								{text.openOrganizations}
							</Button>
							{#if controller.canManage && !controller.organizationEdit.isEditing}
								<OrgchartAddOrganizationPopover
									bind:isOpen={controller.isAddingGroup}
									bind:newGroupName={controller.newGroupName}
									bind:newGroupParentID={controller.newGroupParentID}
									groups={controller.groups}
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
						{#if controller.canManage && !controller.organizationEdit.isEditing}
							<OrgchartAddOrganizationPopover
								bind:isOpen={controller.isAddingGroup}
								bind:newGroupName={controller.newGroupName}
								bind:newGroupParentID={controller.newGroupParentID}
								groups={controller.groups}
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

		<section class="min-h-0 overflow-hidden">
			{#if controller.isLoading}
				<p class="p-6 text-sm text-muted-foreground">{text.loading}</p>
			{:else}
				<div class="grid h-full min-h-0 lg:grid-cols-[270px_minmax(0,1fr)]" data-testid="orgchart-board">
					<div class="hidden min-h-0 lg:block">
						<OrgchartOrganizationTree
							tree={controller.organizationTree}
							selectedOrganizationID={controller.groupID}
							canManage={controller.canManage}
							isEditing={controller.organizationEdit.isEditing}
							isSaving={controller.isSavingGroups}
							{text}
							onSelect={selectOrganization}
							onBeginEdit={() => controller.beginOrganizationEdit()}
							onCancelEdit={() => controller.cancelOrganizationEdit()}
							onSaveEdit={() => controller.saveOrganizationEdit()}
							onMove={(groupID, insertionIndex, depth) => controller.moveOrganization(groupID, insertionIndex, depth)}
						/>
					</div>

					<div class={['grid min-h-0 min-w-0', controller.selectedRecord && !detailSheetViewport.current ? 'grid-cols-[minmax(0,1fr)_320px] 2xl:grid-cols-[minmax(0,1fr)_360px]' : 'grid-cols-1']}>
						<div class="grid min-h-0 min-w-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden">
							<div class="flex min-h-20 items-center justify-between border-b px-4 sm:px-6">
								<h2 class="truncate text-2xl font-semibold">{controller.selectedOrganizationName}</h2>
							</div>
							<div class="min-h-0 overflow-y-auto px-4 py-4 sm:px-6" data-testid="orgchart-list-scroll">
								{#if controller.errorMessage}
									<p class="mb-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{controller.errorMessage}</p>
								{/if}
									{#if controller.organizationSections.every((section) => section.records.length === 0)}
									<p class="rounded-md border bg-muted/30 px-4 py-5 text-sm text-muted-foreground">{text.empty}</p>
								{:else}
									<OrgchartPeopleLayer
										sections={controller.organizationSections}
										selectedUserID={controller.selectedUserID}
										{text}
										selectRecord={(record) => controller.selectRecord(record)}
									/>
								{/if}
							</div>
						</div>

						{#if controller.selectedRecord && !detailSheetViewport.current}
							<div class="min-h-0 border-l p-3" data-testid="orgchart-detail-column">
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
				</div>
			{/if}
		</section>
	</div>

	<Sheet.Root bind:open={isOrganizationSheetOpen}>
		<Sheet.Content side="left" class="w-[min(20rem,90vw)] p-0 lg:hidden" showCloseButton={false}>
			<Sheet.Header class="sr-only"><Sheet.Title>{text.openOrganizations}</Sheet.Title><Sheet.Description>{text.allOrganizations}</Sheet.Description></Sheet.Header>
			<OrgchartOrganizationTree
				tree={controller.organizationTree}
				selectedOrganizationID={controller.groupID}
				canManage={controller.canManage}
				isEditing={controller.organizationEdit.isEditing}
				isSaving={controller.isSavingGroups}
				{text}
				onSelect={selectOrganization}
				onBeginEdit={() => controller.beginOrganizationEdit()}
				onCancelEdit={() => controller.cancelOrganizationEdit()}
				onSaveEdit={() => controller.saveOrganizationEdit()}
				onMove={(groupID, insertionIndex, depth) => controller.moveOrganization(groupID, insertionIndex, depth)}
			/>
		</Sheet.Content>
	</Sheet.Root>

	{#if controller.selectedRecord && detailSheetViewport.current}
		<Sheet.Root open={isDetailSheetOpen} onOpenChange={handleDetailSheetOpenChange}>
			<Sheet.Content side="bottom" class="max-h-[85svh] overflow-hidden rounded-t-xl p-0 lg:hidden" showCloseButton={false} closeLabel={text.closeDetail} data-testid="orgchart-mobile-detail-sheet">
				<Sheet.Header class="sr-only"><Sheet.Title>{text.personDetail}</Sheet.Title><Sheet.Description>{controller.selectedRecord.name || controller.selectedRecord.email}</Sheet.Description></Sheet.Header>
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
