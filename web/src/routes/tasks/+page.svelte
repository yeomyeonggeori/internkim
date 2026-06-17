<script lang="ts">
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import * as Table from '$lib/components/ui/table';
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { goto } from '$app/navigation';
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { onMount } from 'svelte';
	import { fetchTaskRuns, type TaskRunSummary } from './tasks-api';
	import { taskStatusBadgeVariant, taskStatusLabel, formatTaskTimestamp } from './tasks-view';
	import { tasksText } from './text';

	const text = createPageText(tasksText);
	const taskPageSize = 15;
	let taskRuns = $state<TaskRunSummary[]>([]);
	let taskPageIndex = $state(0);
	let totalTaskRunCount = $state(0);
	let statusFilter = $state('');
	let loadError = $state('');
	let isLoading = $state(false);
	let isAdmin = $state(false);
	let taskPageCount = $derived(Math.max(1, Math.ceil(totalTaskRunCount / taskPageSize)));
	let hasNextTaskPage = $derived(taskPageIndex + 1 < taskPageCount);

	const statusFilters = $derived([
		{ value: '', label: text.statusAll },
		{ value: 'failed', label: text.statusFailed },
		{ value: 'running', label: text.statusRunning },
		{ value: 'completed', label: text.statusCompleted }
	]);

	async function loadTaskRuns(pageIndex: number = taskPageIndex) {
		isLoading = true;
		loadError = '';
		try {
			const response = await fetchTaskRuns({
				status: statusFilter || undefined,
				limit: taskPageSize,
				offset: pageIndex * taskPageSize,
				includeTotal: true
			});
			const minimumTotalCount = pageIndex * taskPageSize + response.taskRuns.length;
			const loadedTotalCount = Math.max(response.totalCount ?? minimumTotalCount, minimumTotalCount);
			const lastPageIndex = Math.max(0, Math.ceil(loadedTotalCount / taskPageSize) - 1);
			if (pageIndex > lastPageIndex && response.taskRuns.length === 0 && loadedTotalCount > 0) {
				await loadTaskRuns(lastPageIndex);
				return;
			}
			taskRuns = response.taskRuns;
			totalTaskRunCount = loadedTotalCount;
			taskPageIndex = pageIndex;
		} catch {
			loadError = text.loadError;
		} finally {
			isLoading = false;
		}
	}

	function selectStatus(value: string) {
		statusFilter = value;
		taskPageIndex = 0;
		void loadTaskRuns(0);
	}

	function goToPreviousTaskPage() {
		if (taskPageIndex === 0 || isLoading) return;
		void loadTaskRuns(taskPageIndex - 1);
	}

	function goToNextTaskPage() {
		if (!hasNextTaskPage || isLoading) return;
		void loadTaskRuns(taskPageIndex + 1);
	}

	async function loadViewerRole() {
		try {
			const response = await fetch('/auth/session', { credentials: 'include' });
			if (!response.ok) return;
			const session = (await response.json()) as { isAdmin?: boolean };
			isAdmin = session.isAdmin === true;
		} catch {
			isAdmin = false;
		}
	}

	onMount(() => {
		void loadViewerRole();
		void loadTaskRuns(0);
	});
</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main class="grid min-h-[calc(100svh-48px)] w-full flex-1 content-start gap-5 overflow-x-hidden px-4 py-4 sm:px-6 sm:py-5 lg:px-8">
	<section class="flex min-w-0 flex-wrap items-start justify-between gap-3">
		<div class="min-w-0">
			<h1 class="flex items-center gap-2 text-xl font-semibold">
				<ActivityIcon class="size-5 text-teal-700" />
				{text.title}
			</h1>
			<p class="mt-1 max-w-full text-sm text-muted-foreground">{text.description}</p>
		</div>
		<button
			type="button"
			class="inline-flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-sm hover:bg-muted"
			onclick={() => void loadTaskRuns(taskPageIndex)}
		>
			<RefreshCwIcon class="size-3.5 {isLoading ? 'animate-spin' : ''}" />
			{text.refresh}
		</button>
	</section>

	<UnderlineTabs.Root value={statusFilter} onValueChange={selectStatus}>
		<UnderlineTabs.List>
			{#each statusFilters as filter (filter.value)}
				<UnderlineTabs.Trigger value={filter.value}>{filter.label}</UnderlineTabs.Trigger>
			{/each}
		</UnderlineTabs.List>
	</UnderlineTabs.Root>

	{#if loadError}
		<p class="text-sm text-red-600">{loadError}</p>
	{:else if taskRuns.length === 0 && !isLoading}
		<p class="text-sm text-muted-foreground">{text.empty}</p>
	{:else}
		<section class="min-w-0 overflow-x-auto rounded-lg border">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						{#if isAdmin}
							<Table.Head class="w-40">{text.columnRequester}</Table.Head>
						{/if}
						<Table.Head>{text.columnRequest}</Table.Head>
						<Table.Head class="w-28">{text.columnStatus}</Table.Head>
						<Table.Head class="w-44 text-right">{text.columnUpdated}</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each taskRuns as taskRun (taskRun.taskRunID)}
						<Table.Row
							class="cursor-pointer"
							onclick={() => void goto(`/tasks/${taskRun.taskRunID}`)}
						>
							{#if isAdmin}
								<Table.Cell class="text-sm whitespace-nowrap">
									{taskRun.requesterDisplayName || taskRun.requesterPersonID || '—'}
								</Table.Cell>
							{/if}
							<Table.Cell class="max-w-0">
								<p class="truncate text-sm">{taskRun.prompt || '—'}</p>
								{#if taskRun.failureReason}
									<p class="truncate text-xs text-destructive">{taskRun.failureReason}</p>
								{/if}
							</Table.Cell>
							<Table.Cell>
								<Badge variant={taskStatusBadgeVariant(taskRun.status)}>
									{taskStatusLabel(taskRun.status, text)}
								</Badge>
							</Table.Cell>
							<Table.Cell class="text-right text-xs whitespace-nowrap text-muted-foreground">
								{formatTaskTimestamp(taskRun.updatedAt)}
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</section>
		<ListPaginationFooter
			totalItems={totalTaskRunCount}
			pageIndex={taskPageIndex}
			pageSize={taskPageSize}
			pageCount={taskPageCount}
			canPreviousPage={taskPageIndex > 0 && !isLoading}
			canNextPage={hasNextTaskPage && !isLoading}
			previousPage={goToPreviousTaskPage}
			nextPage={goToNextTaskPage}
			summary={text.paginationSummary}
			previousLabel={text.paginationPrevious}
			nextLabel={text.paginationNext}
			ariaLabel={text.paginationLabel}
		/>
	{/if}
</main>
