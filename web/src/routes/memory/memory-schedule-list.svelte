<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Pagination from '$lib/components/ui/pagination';
	import * as Select from '$lib/components/ui/select';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Switch } from '$lib/components/ui/switch';
	import * as Table from '$lib/components/ui/table';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import { onMount } from 'svelte';
	import {
		deleteSchedule as deleteMemorySchedule,
		fetchMemorySchedules,
		updateSchedule,
		type MemorySchedule,
		type ScheduleUpdateFields
	} from './memory-schedule-api';
	import {
		formatScheduleCronExpression,
		formatScheduleDateTime,
		formatScheduleInterval
	} from './memory-schedule-format';
	import type { MemoryText } from './text';

	type ScheduleKind = 'once' | 'interval' | 'cron';
	type RepeatPolicy = 'finite' | 'unbounded';

	type ScheduleEditDraft = {
		taskScheduleID: string;
		name: string;
		kind: ScheduleKind;
		runAt: string;
		intervalMinute: string;
		cronExpression: string;
		timeZone: string;
		expiresAt: string;
		maxRunCount: string;
		repeatPolicy: RepeatPolicy;
	};

	let { text }: { text: MemoryText } = $props();

	const defaultPageSize = 25;
	const emptyDraft: ScheduleEditDraft = {
		taskScheduleID: '',
		name: '',
		kind: 'once',
		runAt: '',
		intervalMinute: '60',
		cronExpression: '',
		timeZone: 'Asia/Seoul',
		expiresAt: '',
		maxRunCount: '',
		repeatPolicy: 'unbounded'
	};

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
	let scheduleDraft = $state<ScheduleEditDraft>({ ...emptyDraft });

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
		scheduleDraft = scheduleEditDraft(schedule);
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
		const fields = scheduleUpdateFields(scheduleDraft);
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

	function scheduleEditDraft(schedule: MemorySchedule): ScheduleEditDraft {
		return {
			taskScheduleID: schedule.taskScheduleID,
			name: schedule.name ?? '',
			kind: normalizedScheduleKind(schedule.kind),
			runAt: dateTimeInputValue(schedule.nextRunAt),
			intervalMinute: schedule.intervalSecond ? String(Math.max(1, Math.round(schedule.intervalSecond / 60))) : '60',
			cronExpression: schedule.cronExpression ?? '',
			timeZone: schedule.timeZone ?? 'Asia/Seoul',
			expiresAt: dateTimeInputValue(schedule.expiresAt),
			maxRunCount: schedule.maxRunCount && schedule.maxRunCount > 0 ? String(schedule.maxRunCount) : '',
			repeatPolicy: schedule.maxRunCount || schedule.expiresAt ? 'finite' : 'unbounded'
		};
	}

	function scheduleUpdateFields(draft: ScheduleEditDraft): ScheduleUpdateFields | undefined {
		const fields: ScheduleUpdateFields = {
			kind: draft.kind
		};
		const name = draft.name.trim();
		if (name) fields.name = name;
		if (draft.kind === 'once') {
			const runAt = dateTimeInputToISOString(draft.runAt);
			if (!runAt) return undefined;
			fields.runAt = runAt;
			return fields;
		}
		if (draft.kind === 'interval') {
			const intervalMinute = positiveInteger(draft.intervalMinute);
			if (!intervalMinute) return undefined;
			fields.intervalSecond = intervalMinute * 60;
		}
		if (draft.kind === 'cron') {
			const cronExpression = draft.cronExpression.trim();
			if (!cronExpression) return undefined;
			fields.cronExpression = cronExpression;
			fields.timeZone = draft.timeZone.trim() || 'Asia/Seoul';
		}
		fields.repeatPolicy = draft.repeatPolicy;
		const expiresAt = dateTimeInputToISOString(draft.expiresAt);
		if (expiresAt) fields.expiresAt = expiresAt;
		const maxRunCount = positiveInteger(draft.maxRunCount);
		if (maxRunCount) fields.maxRunCount = maxRunCount;
		return fields;
	}

	function canSaveSchedule(): boolean {
		return Boolean(scheduleUpdateFields(scheduleDraft)) && !isSavingSchedule;
	}

	function scheduleTitle(schedule: MemorySchedule): string {
		return schedule.promptPreview?.trim() || schedule.name?.trim() || schedule.taskScheduleID;
	}

	function scheduleKind(schedule: MemorySchedule): string {
		return scheduleKindLabel(normalizedScheduleKind(schedule.kind));
	}

	function scheduleStatusLabel(schedule: MemorySchedule): string {
		if (isExpiredSchedule(schedule)) return text.scheduleStatusExpired;
		if (isCompletedSchedule(schedule)) return text.scheduleStatusCompleted;
		return text.scheduleStatusActive;
	}

	function scheduleStatusVariant(schedule: MemorySchedule): 'default' | 'secondary' | 'destructive' | 'outline' {
		if (isExpiredSchedule(schedule)) return 'destructive';
		if (isCompletedSchedule(schedule)) return 'outline';
		return 'secondary';
	}

	function isCompletedSchedule(schedule: MemorySchedule): boolean {
		return !schedule.nextRunAt;
	}

	function isExpiredSchedule(schedule: MemorySchedule): boolean {
		if (!schedule.expiresAt) return false;
		const expiresAt = Date.parse(schedule.expiresAt);
		return Number.isFinite(expiresAt) && expiresAt <= Date.now();
	}

	function scheduleKindLabel(kind: ScheduleKind): string {
		if (kind === 'cron') return text.scheduleKindCron;
		if (kind === 'interval') return text.scheduleKindInterval;
		return text.scheduleKindOnce;
	}

	function normalizedScheduleKind(kind: string): ScheduleKind {
		if (kind === 'cron') return 'cron';
		if (kind === 'interval') return 'interval';
		return 'once';
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

	function expirationText(schedule: MemorySchedule): string {
		return formatScheduleDateTime(schedule.expiresAt, schedule.timeZone, dateTimeLocale()) ?? text.scheduleNoExpiration;
	}

	function runCountText(schedule: MemorySchedule): string {
		const completedRunCount = schedule.completedRunCount ?? 0;
		if (!schedule.maxRunCount) return text.scheduleRunCountUnlimitedTemplate.replace('{count}', String(completedRunCount));
		return text.scheduleRunCountLimitedTemplate
			.replace('{count}', String(completedRunCount))
			.replace('{limit}', String(schedule.maxRunCount));
	}

	function failureCountText(schedule: MemorySchedule): string {
		return text.scheduleFailureCountTemplate.replace('{count}', String(schedule.failureCount ?? 0));
	}

	function dateTimeLocale(): string {
		return currentLocale.value === 'ko' ? 'ko-KR' : 'en-US';
	}

	function pageSummary(): string {
		const start = totalCount === 0 ? 0 : (currentPage - 1) * pageSize + 1;
		const end = Math.min(currentPage * pageSize, totalCount);
		return text.schedulePageSummaryTemplate.replace('{start}', String(start)).replace('{end}', String(end)).replace('{total}', String(totalCount));
	}

	function positiveInteger(value: string): number | undefined {
		const number = Number(value);
		if (!Number.isFinite(number) || number <= 0) return undefined;
		return Math.floor(number);
	}

	function dateTimeInputValue(value: string | undefined): string {
		if (!value) return '';
		const date = new Date(value);
		if (!Number.isFinite(date.getTime())) return '';
		const year = date.getFullYear();
		const month = paddedDatePart(date.getMonth() + 1);
		const day = paddedDatePart(date.getDate());
		const hour = paddedDatePart(date.getHours());
		const minute = paddedDatePart(date.getMinutes());
		return `${year}-${month}-${day}T${hour}:${minute}`;
	}

	function dateTimeInputToISOString(value: string): string | undefined {
		if (!value.trim()) return undefined;
		const date = new Date(value);
		if (!Number.isFinite(date.getTime())) return undefined;
		return date.toISOString();
	}

	function paddedDatePart(value: number): string {
		return String(value).padStart(2, '0');
	}
</script>

<section class="min-w-0">
	<Card.Root size="sm" class="rounded-lg">
		<Card.Header>
			<Card.Title>{text.scheduleTab}</Card.Title>
			<Card.Description>{text.scheduleDescription}</Card.Description>
			<Card.Action class="flex flex-wrap items-center justify-end gap-2">
				<div class="flex items-center gap-2 rounded-md border px-2.5 py-2">
					<Switch
						id="memory-include-expired"
						size="sm"
						checked={includeExpiredSchedules}
						onCheckedChange={toggleIncludeExpiredSchedules}
					/>
					<Label for="memory-include-expired" class="whitespace-nowrap text-xs font-medium">
						{text.scheduleIncludeExpired}
					</Label>
				</div>
				<Button type="button" variant="outline" size="sm" disabled={isLoading} onclick={refreshSchedules} class="gap-2">
					{#if isLoading}
						<LoaderIcon class="size-4 animate-spin" />
					{:else}
						<RefreshCwIcon class="size-4" />
					{/if}
					{text.refresh}
				</Button>
			</Card.Action>
		</Card.Header>

		<Card.Content class="grid gap-3">
			{#if hasLoadError}
				<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{text.scheduleLoadFailed}</p>
			{/if}
			{#if actionErrorMessage}
				<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{actionErrorMessage}</p>
			{/if}

			{#if isLoading && schedules.length === 0}
				<div class="grid gap-3 rounded-lg border p-4">
					<Skeleton class="h-5 w-1/3" />
					<Skeleton class="h-10 w-full" />
					<Skeleton class="h-10 w-full" />
					<Skeleton class="h-10 w-full" />
				</div>
			{:else if schedules.length === 0 && !hasLoadError}
				<p class="rounded-md border bg-muted/30 px-3 py-12 text-center text-sm text-muted-foreground">{text.scheduleEmpty}</p>
			{:else if schedules.length > 0}
				<div class="overflow-x-auto rounded-lg border">
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head class="min-w-72">{text.schedulePrompt}</Table.Head>
								<Table.Head>{text.scheduleStatus}</Table.Head>
								<Table.Head>{text.scheduleKind}</Table.Head>
								<Table.Head class="min-w-44">{text.scheduleTiming}</Table.Head>
								<Table.Head>{text.scheduleNextRun}</Table.Head>
								<Table.Head>{text.scheduleExpiresAt}</Table.Head>
								<Table.Head>{text.scheduleRunCount}</Table.Head>
								<Table.Head>{text.scheduleFailures}</Table.Head>
								<Table.Head class="min-w-36 text-right">{text.scheduleActions}</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each schedules as schedule}
								<Table.Row>
									<Table.Cell class="max-w-96">
										<p class="truncate font-medium">{scheduleTitle(schedule)}</p>
										<p class="truncate text-xs text-muted-foreground">{schedule.taskScheduleID}</p>
									</Table.Cell>
									<Table.Cell>
										<Badge variant={scheduleStatusVariant(schedule)}>{scheduleStatusLabel(schedule)}</Badge>
									</Table.Cell>
									<Table.Cell>
										<Badge variant="secondary">{scheduleKind(schedule)}</Badge>
									</Table.Cell>
									<Table.Cell class="max-w-72 break-words text-muted-foreground">{scheduleTiming(schedule)}</Table.Cell>
									<Table.Cell class="whitespace-nowrap tabular-nums">{dateTimeText(schedule.nextRunAt, schedule.timeZone)}</Table.Cell>
									<Table.Cell class="whitespace-nowrap tabular-nums">{expirationText(schedule)}</Table.Cell>
									<Table.Cell class="whitespace-nowrap tabular-nums">{runCountText(schedule)}</Table.Cell>
									<Table.Cell>
										<Badge variant={(schedule.failureCount ?? 0) > 0 ? 'destructive' : 'outline'}>{failureCountText(schedule)}</Badge>
									</Table.Cell>
									<Table.Cell>
										<div class="flex justify-end gap-1">
											<Button type="button" variant="ghost" size="sm" onclick={() => openEditDialog(schedule)} class="gap-1">
												<PencilIcon class="size-3.5" />
												{text.scheduleEdit}
											</Button>
											<Button type="button" variant="destructive" size="sm" onclick={() => confirmDeleteSchedule(schedule)} class="gap-1">
												<Trash2Icon class="size-3.5" />
												{text.scheduleDelete}
											</Button>
										</div>
									</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</div>
			{/if}
		</Card.Content>

		{#if !hasLoadError && !(isLoading && schedules.length === 0)}
			<Card.Footer class="flex flex-col items-center justify-between gap-3 sm:flex-row">
				<p class="text-sm text-muted-foreground">{pageSummary()}</p>
				<Pagination.Root
					count={totalCount}
					perPage={pageSize}
					page={currentPage}
					onPageChange={changeSchedulePage}
					aria-label={text.schedulePagination}
				>
					{#snippet children({ pages, currentPage })}
						<Pagination.Content>
							<Pagination.Item>
								<Pagination.PrevButton aria-label={text.schedulePreviousPage}>{text.schedulePreviousPage}</Pagination.PrevButton>
							</Pagination.Item>
							{#each pages as page (page.key)}
								{#if page.type === 'ellipsis'}
									<Pagination.Item>
										<Pagination.Ellipsis />
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
								<Pagination.NextButton aria-label={text.scheduleNextPage}>{text.scheduleNextPage}</Pagination.NextButton>
							</Pagination.Item>
						</Pagination.Content>
					{/snippet}
				</Pagination.Root>
			</Card.Footer>
		{/if}
	</Card.Root>
</section>

<Dialog.Root bind:open={isEditDialogOpen}>
	<Dialog.Content class="max-w-xl">
		<form
			class="grid gap-5"
			onsubmit={(event) => {
				event.preventDefault();
				void saveSchedule();
			}}
		>
			<Dialog.Header>
				<Dialog.Title>{text.scheduleEditTitle}</Dialog.Title>
				<Dialog.Description>{text.scheduleEditDescription}</Dialog.Description>
			</Dialog.Header>

			<div class="grid gap-4">
				<div class="grid gap-2">
					<Label for="schedule-name">{text.scheduleNameLabel}</Label>
					<Input id="schedule-name" bind:value={scheduleDraft.name} />
				</div>

				<div class="grid gap-2">
					<Label>{text.scheduleKind}</Label>
					<Select.Root type="single" bind:value={scheduleDraft.kind}>
						<Select.Trigger class="w-full">{scheduleKindLabel(scheduleDraft.kind)}</Select.Trigger>
						<Select.Content>
							<Select.Item value="once" label={text.scheduleKindOnce}>{text.scheduleKindOnce}</Select.Item>
							<Select.Item value="interval" label={text.scheduleKindInterval}>{text.scheduleKindInterval}</Select.Item>
							<Select.Item value="cron" label={text.scheduleKindCron}>{text.scheduleKindCron}</Select.Item>
						</Select.Content>
					</Select.Root>
				</div>

				{#if scheduleDraft.kind === 'once'}
					<div class="grid gap-2">
						<Label for="schedule-run-at">{text.scheduleRunAtLabel}</Label>
						<Input id="schedule-run-at" type="datetime-local" bind:value={scheduleDraft.runAt} />
					</div>
				{:else if scheduleDraft.kind === 'interval'}
					<div class="grid gap-2">
						<Label for="schedule-interval-minute">{text.scheduleIntervalMinuteLabel}</Label>
						<Input id="schedule-interval-minute" type="number" min="1" step="1" bind:value={scheduleDraft.intervalMinute} />
					</div>
				{:else}
					<div class="grid gap-2">
						<Label for="schedule-cron-expression">{text.scheduleCronExpressionLabel}</Label>
						<Input id="schedule-cron-expression" bind:value={scheduleDraft.cronExpression} placeholder="0 9 * * *" />
					</div>
					<div class="grid gap-2">
						<Label for="schedule-time-zone">{text.scheduleTimeZoneLabel}</Label>
						<Input id="schedule-time-zone" bind:value={scheduleDraft.timeZone} placeholder="Asia/Seoul" />
					</div>
				{/if}

				{#if scheduleDraft.kind !== 'once'}
					<div class="grid gap-2">
						<Label>{text.scheduleRepeatPolicyLabel}</Label>
						<Select.Root type="single" bind:value={scheduleDraft.repeatPolicy}>
							<Select.Trigger class="w-full">{scheduleDraft.repeatPolicy === 'unbounded' ? text.scheduleRepeatUnbounded : text.scheduleRepeatFinite}</Select.Trigger>
							<Select.Content>
								<Select.Item value="unbounded" label={text.scheduleRepeatUnbounded}>{text.scheduleRepeatUnbounded}</Select.Item>
								<Select.Item value="finite" label={text.scheduleRepeatFinite}>{text.scheduleRepeatFinite}</Select.Item>
							</Select.Content>
						</Select.Root>
					</div>
					<div class="grid gap-2 sm:grid-cols-2">
						<div class="grid gap-2">
							<Label for="schedule-expires-at">{text.scheduleExpiresAt}</Label>
							<Input id="schedule-expires-at" type="datetime-local" bind:value={scheduleDraft.expiresAt} />
						</div>
						<div class="grid gap-2">
							<Label for="schedule-max-run-count">{text.scheduleMaxRunCountLabel}</Label>
							<Input id="schedule-max-run-count" type="number" min="1" step="1" bind:value={scheduleDraft.maxRunCount} />
						</div>
					</div>
				{/if}
			</div>

			<Dialog.Footer>
				<Button type="button" variant="outline" onclick={() => (isEditDialogOpen = false)}>{text.cancel}</Button>
				<Button type="submit" disabled={!canSaveSchedule()} class="gap-2">
					{#if isSavingSchedule}
						<LoaderIcon class="size-4 animate-spin" />
					{/if}
					{text.save}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
