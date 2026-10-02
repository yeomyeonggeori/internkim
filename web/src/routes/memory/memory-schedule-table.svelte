<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as Table from '$lib/components/ui/table';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import { MediaQuery } from 'svelte/reactivity';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import type { MemorySchedule } from './memory-schedule-api';
	import { normalizedScheduleKind, scheduleKindLabel } from './memory-schedule-draft';
	import {
		formatScheduleCronExpression,
		formatScheduleDateTime,
		formatScheduleInterval
	} from './memory-schedule-format';
	import type { MemoryText } from './text';

	type Props = {
		text: MemoryText;
		schedules: MemorySchedule[];
		isLoading: boolean;
		hasLoadError: boolean;
		openEditDialog: (schedule: MemorySchedule) => void;
		confirmDeleteSchedule: (schedule: MemorySchedule) => void;
	};

	let { text, schedules, isLoading, hasLoadError, openEditDialog, confirmDeleteSchedule }: Props = $props();
	const isMobile = new MediaQuery('(max-width: 639px)');

	function scheduleTitle(schedule: MemorySchedule): string {
		return schedule.promptPreview?.trim() || schedule.name?.trim() || schedule.taskScheduleID;
	}

	function scheduleKind(schedule: MemorySchedule): string {
		return scheduleKindLabel(text, normalizedScheduleKind(schedule.kind));
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
</script>

{#if isLoading && schedules.length === 0}
	<div class="grid gap-2 p-4">
		<Skeleton class="h-9 w-full" />
		<Skeleton class="h-9 w-full" />
		<Skeleton class="h-9 w-full" />
	</div>
{:else if schedules.length === 0 && !hasLoadError}
	<div class="grid place-items-center px-4 py-16">
		<p class="text-sm text-muted-foreground">{text.scheduleEmpty}</p>
	</div>
{:else if schedules.length > 0}
	{#if isMobile.current}
		<ul class="divide-y">
			{#each schedules as schedule (schedule.taskScheduleID)}
				<li class="grid min-w-0 gap-3 p-4">
					<p class="break-words text-sm font-medium">{scheduleTitle(schedule)}</p>
					<div class="flex flex-wrap items-center gap-2"><Badge variant={scheduleStatusVariant(schedule)}>{scheduleStatusLabel(schedule)}</Badge><span class="text-sm text-muted-foreground">{scheduleTiming(schedule)}</span></div>
					<p class="text-sm">{text.scheduleNextRun}: {dateTimeText(schedule.nextRunAt, schedule.timeZone)}</p>
					<div class="flex flex-wrap items-center gap-2">
						<Button variant="outline" onclick={() => openEditDialog(schedule)}><PencilIcon />{text.scheduleEdit}</Button>
						<Button variant="ghost" class="text-destructive" onclick={() => confirmDeleteSchedule(schedule)}><Trash2Icon />{text.scheduleDelete}</Button>
					</div>
					<Collapsible.Root>
						<Collapsible.Trigger>{#snippet child({ props })}<Button {...props} variant="ghost" class="px-0 text-muted-foreground">{text.scheduleDetails}</Button>{/snippet}</Collapsible.Trigger>
						<Collapsible.Content><dl class="grid gap-3 border-t pt-3 text-sm">
							<div><dt class="text-xs text-muted-foreground">ID</dt><dd class="break-all font-mono">{schedule.taskScheduleID}</dd></div>
							<div><dt class="text-xs text-muted-foreground">{text.scheduleKind}</dt><dd>{scheduleKind(schedule)}</dd></div>
							<div><dt class="text-xs text-muted-foreground">{text.scheduleExpiresAt}</dt><dd>{expirationText(schedule)}</dd></div>
							<div><dt class="text-xs text-muted-foreground">{text.scheduleRunCount}</dt><dd>{runCountText(schedule)}</dd></div>
							<div><dt class="text-xs text-muted-foreground">{text.scheduleFailures}</dt><dd>{failureCountText(schedule)}</dd></div>
						</dl></Collapsible.Content>
					</Collapsible.Root>
				</li>
			{/each}
		</ul>
	{:else}
	<div class="overflow-x-auto">
		<Table.Root>
			<Table.Header>
				<Table.Row class="hover:bg-transparent">
					<Table.Head class="min-w-80 px-4 text-xs text-muted-foreground">{text.schedulePrompt}</Table.Head>
					<Table.Head class="px-3 text-xs text-muted-foreground">{text.scheduleStatus}</Table.Head>
					<Table.Head class="px-3 text-xs text-muted-foreground">{text.scheduleKind}</Table.Head>
					<Table.Head class="min-w-44 px-3 text-xs text-muted-foreground">{text.scheduleTiming}</Table.Head>
					<Table.Head class="px-3 text-xs text-muted-foreground">{text.scheduleNextRun}</Table.Head>
					<Table.Head class="px-3 text-xs text-muted-foreground">{text.scheduleExpiresAt}</Table.Head>
					<Table.Head class="px-3 text-xs text-muted-foreground">{text.scheduleRunCount}</Table.Head>
					<Table.Head class="px-3 text-xs text-muted-foreground">{text.scheduleFailures}</Table.Head>
					<Table.Head class="w-20 px-4 text-right text-xs text-muted-foreground">{text.scheduleActions}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each schedules as schedule}
					<Table.Row>
						<Table.Cell class="max-w-96 px-4 py-3">
							<p class="truncate text-sm font-medium">{scheduleTitle(schedule)}</p>
							<p class="mt-0.5 truncate font-mono text-xs text-muted-foreground">{schedule.taskScheduleID}</p>
						</Table.Cell>
						<Table.Cell class="px-3 py-3">
							<Badge variant={scheduleStatusVariant(schedule)}>{scheduleStatusLabel(schedule)}</Badge>
						</Table.Cell>
						<Table.Cell class="px-3 py-3">
							<Badge variant="secondary">{scheduleKind(schedule)}</Badge>
						</Table.Cell>
						<Table.Cell class="max-w-72 px-3 py-3 text-sm text-muted-foreground">{scheduleTiming(schedule)}</Table.Cell>
						<Table.Cell class="px-3 py-3 text-sm tabular-nums">{dateTimeText(schedule.nextRunAt, schedule.timeZone)}</Table.Cell>
						<Table.Cell class="px-3 py-3 text-sm tabular-nums">{expirationText(schedule)}</Table.Cell>
						<Table.Cell class="px-3 py-3 text-sm tabular-nums">{runCountText(schedule)}</Table.Cell>
						<Table.Cell class="px-3 py-3">
							<Badge variant={(schedule.failureCount ?? 0) > 0 ? 'destructive' : 'outline'}>{failureCountText(schedule)}</Badge>
						</Table.Cell>
						<Table.Cell class="px-4 py-3">
							<div class="flex justify-end gap-1">
								<Button type="button" variant="ghost" size="icon-sm" onclick={() => openEditDialog(schedule)} aria-label={text.scheduleEdit} title={text.scheduleEdit}>
									<PencilIcon class="size-4" />
									<span class="sr-only">{text.scheduleEdit}</span>
								</Button>
								<Button
									type="button"
									variant="ghost"
									size="icon-sm"
									onclick={() => confirmDeleteSchedule(schedule)}
									aria-label={text.scheduleDelete}
									title={text.scheduleDelete}
									class="text-destructive hover:bg-destructive/10 hover:text-destructive"
								>
									<Trash2Icon class="size-4" />
									<span class="sr-only">{text.scheduleDelete}</span>
								</Button>
							</div>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
	{/if}
{/if}
