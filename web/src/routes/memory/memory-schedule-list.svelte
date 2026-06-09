<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Pagination from '$lib/components/ui/pagination';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { onMount } from 'svelte';
	import { fetchMemorySchedules, type MemorySchedule } from './memory-schedule-api';
	import {
		formatScheduleCronExpression,
		formatScheduleDateTime,
		formatScheduleInterval
	} from './memory-schedule-format';
	import type { MemoryText } from './text';

	let { text }: { text: MemoryText } = $props();

	const defaultPageSize = 25;

	let schedules = $state<MemorySchedule[]>([]);
	let currentPage = $state(1);
	let pageSize = $state(defaultPageSize);
	let totalCount = $state(0);
	let hasLoadError = $state(false);
	let isLoading = $state(false);

	let shouldShowPagination = $derived(totalCount > pageSize);

	onMount(() => {
		void loadSchedules(currentPage);
	});

	async function loadSchedules(page: number): Promise<void> {
		isLoading = true;
		hasLoadError = false;
		try {
			const response = await fetchMemorySchedules({ page, pageSize });
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
		void loadSchedules(currentPage);
	}

	function changeSchedulePage(page: number): void {
		if (page === currentPage || isLoading) return;
		currentPage = page;
		void loadSchedules(page);
	}

	function scheduleTitle(schedule: MemorySchedule): string {
		return schedule.promptPreview?.trim() || schedule.taskScheduleID;
	}

	function scheduleStatus(schedule: MemorySchedule): string {
		if (!schedule.nextRunAt) return text.scheduleStatusInactive;
		if ((schedule.failureCount ?? 0) > 0) return text.scheduleStatusRetrying;
		return text.scheduleStatusActive;
	}

	function scheduleKind(schedule: MemorySchedule): string {
		if (schedule.kind === 'cron') return text.scheduleKindCron;
		if (schedule.kind === 'interval') return text.scheduleKindInterval;
		if (schedule.kind === 'once') return text.scheduleKindOnce;
		return text.scheduleKindUnknown;
	}

	function scheduleTiming(schedule: MemorySchedule): string {
		if (schedule.kind === 'cron' && schedule.cronExpression) {
			return formatScheduleCronExpression(schedule.cronExpression, currentLocale.value, text);
		}
		if (schedule.kind === 'interval' && schedule.intervalSecond) return formatScheduleInterval(schedule.intervalSecond, text);
		if (schedule.kind === 'once') return text.scheduleOnce;
		return text.scheduleTimingUnavailable;
	}

	function dateTimeText(value: string | undefined, timeZone: string | undefined): string {
		return formatScheduleDateTime(value, timeZone, dateTimeLocale()) ?? text.scheduleTimeUnavailable;
	}

	function dateTimeLocale(): string {
		return currentLocale.value === 'ko' ? 'ko-KR' : 'en-US';
	}

	function pageSummary(): string {
		const start = totalCount === 0 ? 0 : (currentPage - 1) * pageSize + 1;
		const end = Math.min(currentPage * pageSize, totalCount);
		return text.schedulePageSummaryTemplate
			.replace('{start}', String(start))
			.replace('{end}', String(end))
			.replace('{total}', String(totalCount));
	}
</script>

<section class="grid min-w-0 gap-3">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<div class="min-w-0">
			<h2 class="text-base font-semibold">{text.scheduleTab}</h2>
			<p class="text-sm text-muted-foreground">{text.scheduleDescription}</p>
		</div>
		<Button type="button" variant="outline" disabled={isLoading} onclick={refreshSchedules} class="gap-2">
			{#if isLoading}
				<LoaderIcon class="size-4 animate-spin" />
			{:else}
				<RefreshCwIcon class="size-4" />
			{/if}
			{text.refresh}
		</Button>
	</div>

	{#if hasLoadError}
		<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{text.scheduleLoadFailed}</p>
	{/if}

	{#if isLoading && schedules.length === 0}
		<p class="rounded-md border bg-muted/30 px-3 py-12 text-center text-sm text-muted-foreground">{text.scheduleLoading}</p>
	{:else if schedules.length === 0 && !hasLoadError}
		<p class="rounded-md border bg-muted/30 px-3 py-12 text-center text-sm text-muted-foreground">{text.scheduleEmpty}</p>
	{:else if schedules.length > 0}
		<div class="overflow-hidden rounded-lg border">
			<div class="hidden grid-cols-[minmax(0,1.5fr)_110px_110px_170px_minmax(140px,1fr)] gap-3 border-b bg-muted/30 px-3 py-2 text-xs font-medium text-muted-foreground md:grid">
				<span>{text.scheduleTitle}</span>
				<span>{text.scheduleStatus}</span>
				<span>{text.scheduleKind}</span>
				<span>{text.scheduleNextRun}</span>
				<span>{text.scheduleTiming}</span>
			</div>
			{#each schedules as schedule}
				<article class="grid gap-2 border-b px-3 py-3 last:border-b-0 md:grid-cols-[minmax(0,1.5fr)_110px_110px_170px_minmax(140px,1fr)] md:items-center md:gap-3">
					<div class="min-w-0">
						<p class="truncate text-sm font-medium">{scheduleTitle(schedule)}</p>
						<p class="truncate text-xs text-muted-foreground">{schedule.taskScheduleID}</p>
					</div>
					<Badge variant="secondary" class="w-fit">{scheduleStatus(schedule)}</Badge>
					<span class="text-sm">{scheduleKind(schedule)}</span>
					<span class="text-sm tabular-nums">{dateTimeText(schedule.nextRunAt, schedule.timeZone)}</span>
					<span class="break-words text-sm text-muted-foreground">{scheduleTiming(schedule)}</span>
				</article>
			{/each}
		</div>
		{#if shouldShowPagination}
			<div class="flex flex-col items-center justify-between gap-3 sm:flex-row">
				<p class="text-sm text-muted-foreground">{pageSummary()}</p>
				<Pagination.Root
					count={totalCount}
					perPage={pageSize}
					page={currentPage}
					onPageChange={changeSchedulePage}
					role="navigation"
					aria-label={text.schedulePagination}
				>
					{#snippet children({ pages, currentPage })}
						<Pagination.Content>
							<Pagination.Item>
								<Pagination.Previous>{text.schedulePreviousPage}</Pagination.Previous>
							</Pagination.Item>
							{#each pages as page (page.key)}
								{#if page.type === 'ellipsis'}
									<Pagination.Item>
										<Pagination.Ellipsis label={text.scheduleMorePages} />
									</Pagination.Item>
								{:else}
									<Pagination.Item>
										<Pagination.Link {page} isActive={currentPage === page.value}>
											{page.value}
										</Pagination.Link>
									</Pagination.Item>
								{/if}
							{/each}
							<Pagination.Item>
								<Pagination.Next>{text.scheduleNextPage}</Pagination.Next>
							</Pagination.Item>
						</Pagination.Content>
					{/snippet}
				</Pagination.Root>
			</div>
		{/if}
	{/if}
</section>
