<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import FilterCombobox from '$lib/components/filter-combobox.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as Sheet from '$lib/components/ui/sheet';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import PanelLeftIcon from '@lucide/svelte/icons/panel-left';
	import { breadcrumbMeta } from '$lib/stores/breadcrumb-meta.svelte';
	import { onMount } from 'svelte';
	import { adminText } from '../admin/text';
	import OrganizationAddOrganizationPopover from './organization-add-organization-popover.svelte';
	import { OrganizationDirectoryController } from './organization-directory-controller.svelte';
	import OrganizationFilterPopover from './organization-filter-popover.svelte';
	import OrganizationOrganizationTree from './organization-tree.svelte';
	import OrganizationPeopleLayer from './organization-people-layer.svelte';
	import OrganizationPersonDetailPanel from './organization-person-detail-panel.svelte';
	import { organizationDirectoryText } from './text';

	const text = createPageText(organizationDirectoryText);
	const adminPageText = createPageText(adminText);
	const adminBaseURL = '/admin/api';
	const detailSheetViewport = new IsMobile(1024);
	const controller = new OrganizationDirectoryController(adminBaseURL, text, adminPageText, () => currentLocale.value);

	const personOptions = $derived(
		controller.records.map((record) => ({
			value: record.name || record.email,
			label: record.name || record.email,
			email: record.email,
			image: record.image ?? ''
		}))
	);
	const isDetailSheetOpen = $derived(detailSheetViewport.current && Boolean(controller.selectedRecord));
	let isOrganizationSheetOpen = $state(false);

	$effect(() => {
		breadcrumbMeta.value = controller.groupID ? controller.selectedOrganizationName : '';
		return () => {
			breadcrumbMeta.value = '';
		};
	});

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
					{#if detailSheetViewport.current}
						<div class="flex shrink-0 items-center gap-2">
							<Button type="button" size="sm" variant="outline" onclick={() => (isOrganizationSheetOpen = true)}>
								<PanelLeftIcon class="size-4" />
								{text.openOrganizations}
							</Button>
							{#if controller.canManage && !controller.organizationEdit.isEditing}
								<OrganizationAddOrganizationPopover
									bind:isOpen={controller.isAddingGroup}
									bind:newGroupName={controller.newGroupName}
									bind:newGroupParentID={controller.newGroupParentID}
									groups={controller.groups}
									isSaving={controller.isSavingGroups}
									text={adminPageText.organization}
									inputID="new-organization-group-mobile"
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
					<FilterCombobox
						bind:value={controller.query}
						options={personOptions}
						label={text.search}
						searchPlaceholder={text.searchPlaceholder}
						class="w-full sm:w-72"
					>
						{#snippet optionContent(option)}
							<PersonAvatar name={option.label} email={option.email} image={option.image} class="size-6" />
							<span class="min-w-0 truncate">{option.label}</span>
						{/snippet}
					</FilterCombobox>
					{#if !detailSheetViewport.current}
						<OrganizationFilterPopover
							bind:isOpen={controller.isFilterOpen}
							selectedGroupID={controller.groupID}
							options={controller.options}
							{text}
							onSelectGroup={(groupID) => controller.selectGroup(groupID)}
						/>
						{#if controller.canManage && !controller.organizationEdit.isEditing}
							<OrganizationAddOrganizationPopover
								bind:isOpen={controller.isAddingGroup}
								bind:newGroupName={controller.newGroupName}
								bind:newGroupParentID={controller.newGroupParentID}
								groups={controller.groups}
								isSaving={controller.isSavingGroups}
								text={adminPageText.organization}
								inputID="new-organization-group"
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
				<div class="grid h-full min-h-0 lg:grid-cols-[270px_minmax(0,1fr)]" data-testid="organization-board">
					<div class="hidden min-h-0 lg:block">
						<OrganizationOrganizationTree
							tree={controller.organizationTree}
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
						<div class="grid min-h-0 min-w-0 overflow-hidden">
							<div class="min-h-0 overflow-y-auto px-4 py-4 sm:px-6" data-testid="organization-list-scroll">
								{#if controller.errorMessage}
									<p class="mb-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{controller.errorMessage}</p>
								{/if}
									{#if controller.organizationSections.every((section) => section.records.length === 0)}
									<p class="rounded-md border bg-muted/30 px-4 py-5 text-sm text-muted-foreground">{text.empty}</p>
								{:else}
									<OrganizationPeopleLayer
										sections={controller.organizationSections}
										selectedUserID={controller.selectedUserID}
										{text}
										selectRecord={(record) => controller.selectRecord(record)}
									/>
								{/if}
							</div>
						</div>

						{#if controller.selectedRecord && !detailSheetViewport.current}
							<div class="min-h-0 border-l p-3" data-testid="organization-detail-column">
								<OrganizationPersonDetailPanel
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
			<OrganizationOrganizationTree
				tree={controller.organizationTree}
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
			<Sheet.Content side="bottom" class="max-h-[85svh] overflow-hidden rounded-t-xl p-0 lg:hidden" showCloseButton={false} closeLabel={text.closeDetail} data-testid="organization-mobile-detail-sheet">
				<Sheet.Header class="sr-only"><Sheet.Title>{text.personDetail}</Sheet.Title><Sheet.Description>{controller.selectedRecord.name || controller.selectedRecord.email}</Sheet.Description></Sheet.Header>
				<OrganizationPersonDetailPanel
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
