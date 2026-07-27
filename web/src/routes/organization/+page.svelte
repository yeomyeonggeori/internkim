<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import FilterCombobox from '$lib/components/filter-combobox.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as Sheet from '$lib/components/ui/sheet';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import PanelLeftIcon from '@lucide/svelte/icons/panel-left';
	import UserRoundIcon from '@lucide/svelte/icons/user-round';
	import ComponentIcon from '@lucide/svelte/icons/component';
	import { breadcrumbMeta } from '$lib/stores/breadcrumb-meta.svelte';
	import { onMount } from 'svelte';
	import { adminText } from '../admin/text';
	import OrganizationAddOrganizationDialog from './organization-add-organization-dialog.svelte';
	import { OrganizationDirectoryController } from './organization-directory-controller.svelte';
	import { unassignedGroupID } from './organization-directory-model';
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
			keywords: [record.jobTitle ?? ''].filter(Boolean),
			jobTitle: record.jobTitle ?? '',
			email: record.email,
			image: record.image ?? ''
		}))
	);
	const organizationOptions = $derived([
		{ value: '', label: text.allOrganizations },
		...controller.options.groups.map((group) => ({ value: group.id, label: group.name })),
		...(controller.options.hasUnassigned ? [{ value: unassignedGroupID, label: text.unassignedTeam }] : [])
	]);
	const isDetailSheetOpen = $derived(Boolean(controller.selectedRecord));
	let isOrganizationSheetOpen = $state(false);

	$effect(() => {
		breadcrumbMeta.value = controller.groupID ? controller.selectedOrganizationName : '';
		breadcrumbMeta.clear = () => controller.selectGroup('');
		return () => {
			breadcrumbMeta.value = '';
			breadcrumbMeta.clear = undefined;
		};
	});

	onMount(() => {
		void controller.load();
	});

	function handleDetailSheetOpenChange(nextOpen: boolean): void {
		if (nextOpen) return;
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
	<div class="grid h-[calc(100vh-3rem)] min-h-0">
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
							onAddOrganization={() => (controller.isAddingGroup = true)}
							onBeginEdit={() => controller.beginOrganizationEdit()}
							onCancelEdit={() => controller.cancelOrganizationEdit()}
							onSaveEdit={() => controller.saveOrganizationEdit()}
							onMove={(groupID, insertionIndex, depth) => controller.moveOrganization(groupID, insertionIndex, depth)}
						/>
					</div>

					<div class="grid min-h-0 min-w-0 grid-cols-1">
						<div class="grid min-h-0 min-w-0 grid-rows-[auto_minmax(0,1fr)] overflow-hidden">
							<div class="flex items-center gap-2 px-4 py-3 sm:px-6">
								{#if detailSheetViewport.current}
									<Button type="button" size="sm" variant="outline" onclick={() => (isOrganizationSheetOpen = true)}>
										<PanelLeftIcon class="size-4" />
										{text.openOrganizations}
									</Button>
								{/if}
								<FilterCombobox
									value={controller.groupID || ''}
									onSelect={(groupID) => controller.selectGroup(groupID)}
									options={organizationOptions}
									label={text.organization}
									searchPlaceholder={text.organization}
									class="ml-auto w-full sm:w-52"
								>
									{#snippet icon()}
										<ComponentIcon class="size-4 shrink-0 opacity-60" />
									{/snippet}
								</FilterCombobox>
								<FilterCombobox
									bind:value={controller.query}
									options={personOptions}
									label={text.selectEmployee}
									searchPlaceholder={text.searchPlaceholder}
									class="w-full sm:w-72"
								>
									{#snippet icon()}
										<UserRoundIcon class="size-4 shrink-0 opacity-60" />
									{/snippet}
									{#snippet optionContent(option)}
										<PersonAvatar name={option.label} email={option.email} image={option.image} class="size-6" />
										<span class="grid min-w-0">
											<span class="truncate">{option.label}</span>
											{#if option.jobTitle}
												<span class="text-muted-foreground truncate text-xs">{option.jobTitle}</span>
											{/if}
										</span>
									{/snippet}
								</FilterCombobox>
							</div>
							<div class="min-h-0 overflow-y-auto px-4 sm:px-6" data-organization-scroll data-testid="organization-list-scroll">
								{#if controller.errorMessage}
									<p class="mb-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{controller.errorMessage}</p>
								{/if}
									{#if controller.organizationSections.every((section) => section.records.length === 0)}
									<p class="rounded-md border bg-muted/30 px-4 py-5 text-sm text-muted-foreground">{text.empty}</p>
								{:else}
									<OrganizationPeopleLayer
										sections={controller.organizationSections}
										selectedUserID={controller.selectedUserID}
										hidesEmptySections={Boolean(controller.query.trim())}
										{text}
										selectRecord={(record) => controller.selectRecord(record)}
									/>
								{/if}
							</div>
						</div>

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
				onAddOrganization={() => (controller.isAddingGroup = true)}
				onBeginEdit={() => controller.beginOrganizationEdit()}
				onCancelEdit={() => controller.cancelOrganizationEdit()}
				onSaveEdit={() => controller.saveOrganizationEdit()}
				onMove={(groupID, insertionIndex, depth) => controller.moveOrganization(groupID, insertionIndex, depth)}
			/>
		</Sheet.Content>
	</Sheet.Root>

	{#if controller.selectedRecord}
		<Sheet.Root open={isDetailSheetOpen} onOpenChange={handleDetailSheetOpenChange}>
			<Sheet.Content
				side="right"
				class="grid w-[min(26rem,92vw)] grid-rows-[auto_minmax(0,1fr)] gap-0 overflow-hidden p-0"
				closeLabel={text.closeDetail}
				data-testid="organization-detail-sheet"
			>
				<Sheet.Header class="border-b px-6 py-4">
					<Sheet.Title>{text.personDetail}</Sheet.Title>
					<Sheet.Description class="sr-only">{controller.selectedRecord.name || controller.selectedRecord.email}</Sheet.Description>
				</Sheet.Header>
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
				/>
			</Sheet.Content>
		</Sheet.Root>
	{/if}
</main>

<OrganizationAddOrganizationDialog
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
