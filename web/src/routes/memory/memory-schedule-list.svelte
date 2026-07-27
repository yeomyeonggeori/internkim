<script lang="ts">
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import { Button } from '$lib/components/ui/button';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { onMount } from 'svelte';
	import {
		deleteSchedule as deleteMemorySchedule,
		fetchMemorySchedules,
		updateSchedule,
		type MemorySchedule
	} from './memory-schedule-api';
	import {
		canSaveScheduleDraft,
		createScheduleEditDraft,
		createScheduleUpdateFields,
		emptyScheduleEditDraft,
		type ScheduleEditDraft
	} from './memory-schedule-draft';
	import MemoryScheduleEditDialog from './memory-schedule-edit-dialog.svelte';
	import MemoryScheduleTable from './memory-schedule-table.svelte';
	import type { MemoryText } from './text';

	let { text }: { text: MemoryText } = $props();

	const defaultPageSize = 15;

	let schedules = $state<MemorySchedule[]>([]);
	let currentPage = $state(1);
	let pageSize = $state(defaultPageSize);
	let totalCount = $state(0);
	let hasLoadError = $state(false);
	let actionErrorMessage = $state('');
	let isLoading = $state(false);
	let isSavingSchedule = $state(false);
	let isEditDialogOpen = $state(false);
	let includeExpiredSchedules = $state(true);
	let scheduleDraft = $state<ScheduleEditDraft>({ ...emptyScheduleEditDraft });
	let canSaveSchedule = $derived(canSaveScheduleDraft(scheduleDraft, isSavingSchedule));

	onMount(() => {
		void loadSchedules(currentPage);
	});

	async function loadSchedules(page: number): Promise<void> {
		isLoading = true;
		hasLoadError = false;
		try {
			const response = await fetchMemorySchedules({ page, pageSize, includeExpired: includeExpiredSchedules });
			const loadedSchedules = response.schedules ?? [];
			const loadedPageSize = response.pageSize && response.pageSize > 0 ? response.pageSize : pageSize;
			const loadedTotalCount = response.totalCount ?? response.count ?? loadedSchedules.length;
			const lastPage = Math.max(1, Math.ceil(loadedTotalCount / loadedPageSize));
			if (page > lastPage && loadedSchedules.length === 0 && loadedTotalCount > 0) {
				await loadSchedules(lastPage);
				return;
			}
			schedules = loadedSchedules;
			currentPage = response.page ?? page;
			pageSize = loadedPageSize;
			totalCount = loadedTotalCount;
		} catch {
			hasLoadError = true;
		} finally {
			isLoading = false;
		}
	}

	function refreshSchedules(): void {
		actionErrorMessage = '';
		void loadSchedules(currentPage);
	}

	function toggleIncludeExpiredSchedules(value: boolean): void {
		if (includeExpiredSchedules === value) return;
		actionErrorMessage = '';
		includeExpiredSchedules = value;
		currentPage = 1;
		void loadSchedules(1);
	}

	function changeSchedulePage(page: number): void {
		if (page === currentPage || isLoading) return;
		void loadSchedules(page);
	}

	function openEditDialog(schedule: MemorySchedule): void {
		actionErrorMessage = '';
		scheduleDraft = createScheduleEditDraft(schedule);
		isEditDialogOpen = true;
	}

	function confirmDeleteSchedule(schedule: MemorySchedule): void {
		confirmDelete({
			title: text.scheduleDeleteTitle,
			description: text.scheduleDeleteDescription.replace('{value}', scheduleTitle(schedule)),
			confirm: { text: text.scheduleDeleteAction },
			cancel: { text: text.cancel },
			onConfirm: async () => {
				await deleteSchedule(schedule.taskScheduleID);
			}
		});
	}

	async function deleteSchedule(taskScheduleID: string): Promise<void> {
		actionErrorMessage = '';
		try {
			await deleteMemorySchedule(taskScheduleID);
			await loadSchedules(currentPage);
		} catch {
			actionErrorMessage = text.scheduleDeleteFailed;
		}
	}

	async function saveSchedule(): Promise<void> {
		const fields = createScheduleUpdateFields(scheduleDraft);
		if (!fields) return;
		isSavingSchedule = true;
		actionErrorMessage = '';
		try {
			await updateSchedule(scheduleDraft.taskScheduleID, fields);
			isEditDialogOpen = false;
			await loadSchedules(currentPage);
		} catch {
			actionErrorMessage = text.scheduleUpdateFailed;
		} finally {
			isSavingSchedule = false;
		}
	}

	function scheduleTitle(schedule: MemorySchedule): string {
		return schedule.promptPreview?.trim() || schedule.name?.trim() || schedule.taskScheduleID;
	}
</script>

<section class="grid min-w-0 gap-4">
	<div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
		<div class="grid gap-1">
			<h2 class="text-lg font-semibold tracking-tight">{text.scheduleTab}</h2>
			<p class="text-sm text-muted-foreground">{text.scheduleDescription}</p>
		</div>
		<div class="flex flex-wrap items-center gap-2">
			<div class="flex h-9 items-center gap-2 rounded-md border bg-background px-3 shadow-xs">
				<Switch
					id="memory-include-expired"
					size="sm"
					checked={includeExpiredSchedules}
					onCheckedChange={toggleIncludeExpiredSchedules}
				/>
				<Label for="memory-include-expired" class="whitespace-nowrap text-sm font-medium">
					{text.scheduleIncludeExpired}
				</Label>
			</div>
			<Button type="button" variant="ghost" size="icon-sm" disabled={isLoading} onclick={refreshSchedules} aria-label={text.refresh} title={text.refresh}>
				<RefreshCwIcon class={isLoading ? 'animate-spin' : ''} />
			</Button>
		</div>
	</div>

	{#if hasLoadError}
		<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{text.scheduleLoadFailed}</p>
	{/if}
	{#if actionErrorMessage}
		<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{actionErrorMessage}</p>
	{/if}

	<div class="overflow-hidden rounded-md border bg-background">
		<MemoryScheduleTable
			{text}
			{schedules}
			{isLoading}
			{hasLoadError}
			{openEditDialog}
			{confirmDeleteSchedule}
		/>

		{#if totalCount > 0 && !hasLoadError && !(isLoading && schedules.length === 0)}
			<div class="border-t bg-muted/20 px-4 py-3">
				<ListPaginationFooter
					totalItems={totalCount}
					pageIndex={currentPage - 1}
					{pageSize}
					pageCount={Math.max(1, Math.ceil(totalCount / pageSize))}
					canPreviousPage={currentPage > 1 && !isLoading}
					canNextPage={currentPage * pageSize < totalCount && !isLoading}
					previousPage={() => changeSchedulePage(currentPage - 1)}
					nextPage={() => changeSchedulePage(currentPage + 1)}
					summary={text.schedulePageSummaryTemplate}
					previousLabel={text.schedulePreviousPage}
					nextLabel={text.scheduleNextPage}
					ariaLabel={text.schedulePagination}
				/>
			</div>
		{/if}
	</div>
</section>

<MemoryScheduleEditDialog
	{text}
	bind:isOpen={isEditDialogOpen}
	bind:scheduleDraft
	{isSavingSchedule}
	{canSaveSchedule}
	saveSchedule={() => void saveSchedule()}
/>
