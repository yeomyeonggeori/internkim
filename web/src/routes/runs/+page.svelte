<script lang="ts">
	import { withReturnPath } from '$lib/return-path';
	import ListPaginationFooter from '$lib/components/list-pagination-footer.svelte';
	import { page } from '$app/state';
	import { pendingApprovalsPathOf, taskRunDetailPathOf } from '$lib/app-shell';
	import { pageActions } from '$lib/components/app-page-actions.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Table from '$lib/components/ui/table';
	import * as UnderlineTabs from '$lib/components/ui/underline-tabs';
	import * as Tabs from '$lib/components/ui/tabs';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import PersonNameCell from '$lib/components/person-name-cell.svelte';
	import { goto } from '$app/navigation';
	import EllipsisVerticalIcon from '@lucide/svelte/icons/ellipsis-vertical';
	import HourglassIcon from '@lucide/svelte/icons/hourglass';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import { onMount } from 'svelte';
	import { deleteTaskRun, fetchTaskRuns, formatCostUSD, type DailyCostScope, type DailyCostSummary, type TaskRunSummary } from './runs-api';
	import { taskStatusLabel, formatTaskTimestamp } from './runs-view';
	import TaskRunStatus from './task-run-status.svelte';
	import { tasksText } from './text';
	import RetryTaskButton from './retry-task-button.svelte';
	import InboundMessages from './inbound-messages.svelte';

	const text = createPageText(tasksText);
	const taskPageSize = 15;
	const dailyCostTaskRunLimit = 500;
	const deletableTaskStatuses = new Set(['completed', 'failed', 'cancelled', 'blocked']);
	let taskRuns = $state<TaskRunSummary[]>([]);
	let dailyCostSummaries = $state<DailyCostSummary[]>([]);
	let dailyCostScope = $state<DailyCostScope | undefined>(undefined);
	let taskPageIndex = $state(0);
	let totalTaskRunCount = $state(0);
	let statusFilter = $state('');
	let loadError = $state('');
	let actionError = $state('');
	let isLoading = $state(false);
	let isAdmin = $state(false);
	let selectedView = $state('tasks');
	let deletingTaskRunIDs = $state<Set<string>>(new Set());
	let taskPageCount = $derived(Math.max(1, Math.ceil(totalTaskRunCount / taskPageSize)));
	let hasNextTaskPage = $derived(taskPageIndex + 1 < taskPageCount);
	let dailyCostRows = $derived(dailyCostSummaries.slice(0, 4));

	const statusFilters = $derived([
		{ value: '', label: text.statusAll },
		{ value: 'failed', label: text.statusFailed },
		{ value: 'running', label: text.statusRunning },
		{ value: 'completed', label: text.statusCompleted }
	]);

	async function loadTaskRuns(pageIndex: number = taskPageIndex) {
		isLoading = true;
		loadError = '';
		actionError = '';
		try {
			const response = await fetchTaskRuns({
				status: statusFilter || undefined,
				limit: taskPageSize,
				offset: pageIndex * taskPageSize,
				includeTotal: true,
				includeCost: true,
				dailyCostTaskRunLimit
			});
			if (response.totalCount === undefined) throw new Error('Task list response is missing totalCount');
			const loadedTotalCount = response.totalCount;
			const lastPageIndex = Math.max(0, Math.ceil(loadedTotalCount / taskPageSize) - 1);
			if (pageIndex > lastPageIndex && response.taskRuns.length === 0 && loadedTotalCount > 0) {
				await loadTaskRuns(lastPageIndex);
				return;
			}
			taskRuns = response.taskRuns;
			dailyCostSummaries = response.dailyCostSummaries ?? [];
			dailyCostScope = response.dailyCostScope;
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

	function canDeleteTaskRun(taskRun: TaskRunSummary): boolean {
		return deletableTaskStatuses.has(taskRun.status) && !deletingTaskRunIDs.has(taskRun.taskRunID);
	}

	function setTaskRunDeleting(taskRunID: string, isDeleting: boolean) {
		const nextTaskRunIDs = new Set(deletingTaskRunIDs);
		if (isDeleting) {
			nextTaskRunIDs.add(taskRunID);
		} else {
			nextTaskRunIDs.delete(taskRunID);
		}
		deletingTaskRunIDs = nextTaskRunIDs;
	}

	function confirmTaskRunDelete(event: MouseEvent, taskRun: TaskRunSummary) {
		event.stopPropagation();
		if (!canDeleteTaskRun(taskRun)) return;
		confirmDelete({
			title: text.deleteTaskTitle,
			description: text.deleteTaskDescription.replace('{id}', taskRun.taskRunID),
			confirm: { text: text.deleteTask },
			cancel: { text: text.cancelDelete },
			onConfirm: () => deleteSelectedTaskRun(taskRun.taskRunID)
		});
	}

	async function deleteSelectedTaskRun(taskRunID: string) {
		setTaskRunDeleting(taskRunID, true);
		actionError = '';
		try {
			await deleteTaskRun(taskRunID);
			const nextPageIndex = taskRuns.length === 1 && taskPageIndex > 0 ? taskPageIndex - 1 : taskPageIndex;
			await loadTaskRuns(nextPageIndex);
		} catch {
			actionError = text.deleteError;
		} finally {
			setTaskRunDeleting(taskRunID, false);
		}
	}

	function formatCostDate(date: string): string {
		const parsed = new Date(`${date}T00:00:00`);
		if (Number.isNaN(parsed.getTime())) return date;
		return parsed.toLocaleDateString();
	}

	function dailyCostScopeLabel(scope: DailyCostScope | undefined): string {
		if (!scope?.isTruncated) return text.dailyCostScopeAll;
		return text.dailyCostScopeLimited.replace('{count}', scope.taskRunCount.toLocaleString());
	}

	function taskPaginationSummary(): string {
		return text.paginationSummary
			.replace('{total}', String(totalTaskRunCount))
			.replace('{from}', totalTaskRunCount === 0 ? '0' : String(taskPageIndex * taskPageSize + 1))
			.replace('{to}', String(Math.min(totalTaskRunCount, (taskPageIndex + 1) * taskPageSize)));
	}

	function taskRunCostLabel(taskRun: TaskRunSummary): string {
		return taskRun.llmCostUSD && taskRun.llmCostUSD > 0 ? formatCostUSD(taskRun.llmCostUSD) : '—';
	}

	function openTaskRun(taskRunID: string) {
		void goto(taskRunDetailPathOf(page.url.pathname, taskRunID));
	}

	function handleTaskRunKeydown(event: KeyboardEvent, taskRunID: string) {
		if (event.target !== event.currentTarget) return;
		if (event.key !== 'Enter' && event.key !== ' ') return;
		event.preventDefault();
		openTaskRun(taskRunID);
	}

	function stopRowActionClick(event: MouseEvent) {
		event.stopPropagation();
	}

	async function loadViewerRole() {
		try {
			const returnPath = `${location.pathname}${location.search}`;
			const response = await fetch(withReturnPath('/auth/session', returnPath), { credentials: 'include' });
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
		return pageActions.setRefresh(() => loadTaskRuns(taskPageIndex));
	});
</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main class="grid min-h-full w-full self-start content-start gap-5 px-4 py-4 sm:px-6 sm:py-5 lg:px-8">
	{#if isAdmin}
		<Tabs.Root bind:value={selectedView}>
			<Tabs.List>
				<Tabs.Trigger value="tasks">{text.viewTasks}</Tabs.Trigger>
				<Tabs.Trigger value="inbound">{text.viewInbound}</Tabs.Trigger>
			</Tabs.List>
		</Tabs.Root>
	{/if}
	{#if isAdmin && selectedView === 'inbound'}
		<InboundMessages {text} />
	{:else}
	{#if dailyCostRows.length > 0}
	<section class="flex min-w-0 flex-col gap-2 rounded-lg border bg-card p-3">
		<p class="text-xs text-muted-foreground">{text.dailyCostTitle} · {dailyCostScopeLabel(dailyCostScope)}</p>
		<div class="grid w-full grid-cols-[minmax(0,1fr)_auto_auto_auto] gap-x-2 gap-y-2 text-sm tabular-nums sm:w-fit sm:grid-cols-[auto_auto_auto_auto] sm:gap-x-8 sm:gap-y-1">
			<span class="text-xs text-muted-foreground">{text.dailyCostDateColumn}</span>
			<span class="text-right text-xs text-muted-foreground">{text.columnCost}</span>
			<span class="text-right text-xs text-muted-foreground">{text.dailyCostTasksColumn}</span>
			<span class="text-right text-xs text-muted-foreground">{text.dailyCostCallsColumn}</span>
			{#each dailyCostRows as summary (summary.date)}
				<span class="text-muted-foreground">{formatCostDate(summary.date)}</span>
				<span class="text-right font-medium">{formatCostUSD(summary.costUSD)}</span>
				<span class="text-right">{summary.taskRunCount.toLocaleString()}</span>
				<span class="text-right">{summary.llmCallCount.toLocaleString()}</span>
			{/each}
		</div>
	</section>
	{/if}

	<div class="flex flex-wrap items-center justify-between gap-3">
		<UnderlineTabs.Root value={statusFilter} onValueChange={selectStatus}>
			<UnderlineTabs.List>
				{#each statusFilters as filter (filter.value)}
					<UnderlineTabs.Trigger value={filter.value}>{filter.label}</UnderlineTabs.Trigger>
				{/each}
			</UnderlineTabs.List>
		</UnderlineTabs.Root>
		<Button href={pendingApprovalsPathOf(page.url.pathname)} variant="outline" size="sm">
			<HourglassIcon data-icon="inline-start" />
			{text.approvalsTitle}
		</Button>
	</div>

	{#if actionError}
		<Card.Root size="sm" class="border-destructive/30">
			<Card.Content class="text-sm text-destructive">{actionError}</Card.Content>
		</Card.Root>
	{/if}

	{#if loadError}
		<Card.Root size="sm" class="border-destructive/30">
			<Card.Content class="text-sm text-destructive">{loadError}</Card.Content>
		</Card.Root>
	{:else if isLoading && taskRuns.length === 0}
		<Card.Root>
			<Card.Content class="flex flex-col gap-2">
				<Skeleton class="h-10 w-full" />
				<Skeleton class="h-10 w-full" />
				<Skeleton class="h-10 w-full" />
			</Card.Content>
		</Card.Root>
	{:else if taskRuns.length === 0 && !isLoading}
		<Card.Root size="sm">
			<Card.Content class="text-sm text-muted-foreground">{text.empty}</Card.Content>
		</Card.Root>
	{:else}
		<Card.Root class="min-w-0">
			<Card.Content class="px-0">
				<div class="divide-y md:hidden" data-task-run-mobile-list>
					{#each taskRuns as taskRun (taskRun.taskRunID)}
						<div
							role="button"
							tabindex="0"
							class="flex cursor-pointer flex-col gap-2 px-4 py-3 hover:bg-muted/50"
							onclick={() => openTaskRun(taskRun.taskRunID)}
							onkeydown={(event) => handleTaskRunKeydown(event, taskRun.taskRunID)}
						>
							<div class="flex min-w-0 items-start justify-between gap-3">
								<div class="min-w-0">
									<p class="line-clamp-2 text-sm font-medium">{taskRun.prompt || '—'}</p>
									{#if isAdmin}
										<PersonNameCell
											name={taskRun.requesterDisplayName ?? ''}
											personID={taskRun.requesterPersonID ?? ''}
											class="mt-1 text-xs font-normal text-muted-foreground [&_[data-slot=avatar]]:size-5"
										/>
									{/if}
								</div>
								<TaskRunStatus status={taskRun.status} label={taskStatusLabel(taskRun.status, text)} class="shrink-0" />
							</div>
							{#if taskRun.failureReason}
								<p class="line-clamp-2 text-xs text-destructive">{taskRun.failureReason}</p>
							{:else if taskRun.result}
								<p class="line-clamp-2 text-xs text-muted-foreground">{taskRun.result}</p>
							{/if}
							<div class="flex items-center justify-between gap-3 text-xs text-muted-foreground">
								<span class="truncate">{formatTaskTimestamp(taskRun.updatedAt)}</span>
								<div class="flex shrink-0 items-center gap-2">
									<span class="font-medium text-foreground tabular-nums">{taskRunCostLabel(taskRun)}</span>
									{#if taskRun.status === 'failed'}
										<RetryTaskButton taskRunID={taskRun.taskRunID} label={text.retryTask} pendingLabel={text.retryingTask} successMessage={text.retrySuccess} errorMessage={text.retryError} />
									{/if}
									{#if deletableTaskStatuses.has(taskRun.status)}
										<DropdownMenu.Root>
											<DropdownMenu.Trigger onclick={stopRowActionClick}>
												{#snippet child({ props })}
													<Button
														{...props}
														variant="ghost"
														size="icon-xs"
														aria-label={text.columnActions}
														title={text.columnActions}
														disabled={deletingTaskRunIDs.has(taskRun.taskRunID)}
													>
														<EllipsisVerticalIcon />
													</Button>
												{/snippet}
											</DropdownMenu.Trigger>
											<DropdownMenu.Content align="end" sideOffset={6}>
												<DropdownMenu.Item variant="destructive" onclick={(event) => confirmTaskRunDelete(event, taskRun)}>
													<Trash2Icon />
													{text.deleteTask}
												</DropdownMenu.Item>
											</DropdownMenu.Content>
										</DropdownMenu.Root>
									{/if}
								</div>
							</div>
						</div>
					{/each}
				</div>
				<div class="hidden min-w-0 overflow-x-auto md:block">
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>{text.columnRequest}</Table.Head>
								{#if isAdmin}
									<Table.Head class="w-44">{text.columnRequester}</Table.Head>
								{/if}
								<Table.Head class="w-32">{text.columnStatus}</Table.Head>
								<Table.Head class="w-28 text-right">{text.columnCost}</Table.Head>
								<Table.Head class="w-44 text-right">{text.columnUpdated}</Table.Head>
								<Table.Head class="w-12 text-right">
									<span class="sr-only">{text.columnActions}</span>
								</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each taskRuns as taskRun (taskRun.taskRunID)}
								<Table.Row
									class="cursor-pointer hover:bg-muted/50"
									onclick={() => openTaskRun(taskRun.taskRunID)}
								>
									<Table.Cell class="max-w-0">
										<div class="flex min-w-0 flex-col gap-1">
											<p class="truncate text-sm font-medium">{taskRun.prompt || '—'}</p>
											{#if taskRun.failureReason}
												<p class="truncate text-xs text-destructive">{taskRun.failureReason}</p>
											{:else if taskRun.result}
												<p class="truncate text-xs text-muted-foreground">{taskRun.result}</p>
											{/if}
										</div>
									</Table.Cell>
									{#if isAdmin}
										<Table.Cell class="whitespace-nowrap text-sm text-muted-foreground">
											<PersonNameCell name={taskRun.requesterDisplayName ?? ''} personID={taskRun.requesterPersonID ?? ''} />
										</Table.Cell>
									{/if}
									<Table.Cell>
										<TaskRunStatus status={taskRun.status} label={taskStatusLabel(taskRun.status, text)} />
									</Table.Cell>
									<Table.Cell class="text-right text-xs whitespace-nowrap">
										{taskRunCostLabel(taskRun)}
									</Table.Cell>
									<Table.Cell class="text-right text-xs whitespace-nowrap text-muted-foreground">
										{formatTaskTimestamp(taskRun.updatedAt)}
									</Table.Cell>
									<Table.Cell class="text-right">
										{#if taskRun.status === 'failed'}
											<RetryTaskButton taskRunID={taskRun.taskRunID} label={text.retryTask} pendingLabel={text.retryingTask} successMessage={text.retrySuccess} errorMessage={text.retryError} />
										{/if}
										{#if deletableTaskStatuses.has(taskRun.status)}
											<DropdownMenu.Root>
												<DropdownMenu.Trigger onclick={stopRowActionClick}>
													{#snippet child({ props })}
														<Button
															{...props}
															variant="ghost"
															size="icon-xs"
															aria-label={text.columnActions}
															title={text.columnActions}
															disabled={deletingTaskRunIDs.has(taskRun.taskRunID)}
														>
															<EllipsisVerticalIcon />
														</Button>
													{/snippet}
												</DropdownMenu.Trigger>
												<DropdownMenu.Content align="end" sideOffset={6}>
													<DropdownMenu.Item variant="destructive" onclick={(event) => confirmTaskRunDelete(event, taskRun)}>
														<Trash2Icon />
														{text.deleteTask}
													</DropdownMenu.Item>
												</DropdownMenu.Content>
											</DropdownMenu.Root>
										{/if}
									</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</div>
			</Card.Content>
		</Card.Root>
		<div class="max-md:pb-[calc(5rem+env(safe-area-inset-bottom))]">
			<ListPaginationFooter
 onPageChange={(page) => void loadTaskRuns(page)}
 disabled={isLoading}
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
		</div>
	{/if}
	{/if}
</main>
