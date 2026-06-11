<script lang="ts">
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import { fetchTaskRuns, type TaskRunSummary } from './tasks-api';
	import { taskStatusBadgeClass, formatTaskTimestamp, shortTaskRunID } from './tasks-view';
	import { tasksText } from './text';

	const text = createPageText(tasksText);
	let taskRuns = $state<TaskRunSummary[]>([]);
	let statusFilter = $state('');
	let loadError = $state('');
	let isLoading = $state(false);

	const statusFilters = $derived([
		{ value: '', label: text.statusAll },
		{ value: 'failed', label: text.statusFailed },
		{ value: 'running', label: text.statusRunning },
		{ value: 'completed', label: text.statusCompleted }
	]);

	async function load() {
		isLoading = true;
		loadError = '';
		try {
			taskRuns = await fetchTaskRuns(statusFilter || undefined);
		} catch {
			loadError = text.loadError;
		} finally {
			isLoading = false;
		}
	}

	function selectStatus(value: string) {
		statusFilter = value;
		void load();
	}

	onMount(load);
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
			onclick={() => void load()}
		>
			<RefreshCwIcon class="size-3.5 {isLoading ? 'animate-spin' : ''}" />
			{text.refresh}
		</button>
	</section>

	<section class="flex flex-wrap gap-1.5">
		{#each statusFilters as filter (filter.value)}
			<button
				type="button"
				class="rounded-full border px-3 py-1 text-xs {statusFilter === filter.value
					? 'border-teal-700 bg-teal-700 text-white'
					: 'text-muted-foreground hover:bg-muted'}"
				onclick={() => selectStatus(filter.value)}
			>
				{filter.label}
			</button>
		{/each}
	</section>

	{#if loadError}
		<p class="text-sm text-red-600">{loadError}</p>
	{:else if taskRuns.length === 0 && !isLoading}
		<p class="text-sm text-muted-foreground">{text.empty}</p>
	{:else}
		<section class="grid gap-2">
			{#each taskRuns as taskRun (taskRun.taskRunID)}
				<a
					href={`/tasks/${taskRun.taskRunID}`}
					class="grid min-w-0 gap-1 rounded-lg border px-4 py-3 hover:bg-muted/50"
				>
					<div class="flex min-w-0 flex-wrap items-center gap-2">
						<code class="text-xs text-muted-foreground">{shortTaskRunID(taskRun.taskRunID)}</code>
						<span class="rounded-full px-2 py-0.5 text-[11px] font-medium {taskStatusBadgeClass(taskRun.status)}">
							{taskRun.status}
						</span>
						<span class="ml-auto text-xs text-muted-foreground">
							{formatTaskTimestamp(taskRun.updatedAt)}
						</span>
					</div>
					<p class="truncate text-sm">{taskRun.prompt || '—'}</p>
					{#if taskRun.failureReason}
						<p class="truncate text-xs text-red-600">{taskRun.failureReason}</p>
					{/if}
				</a>
			{/each}
		</section>
	{/if}
</main>
