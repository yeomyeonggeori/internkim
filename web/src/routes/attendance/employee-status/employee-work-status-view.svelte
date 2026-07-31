<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import {
		type AttendanceEmployeeWorkStatus,
		type AttendanceWorkStatusPeriod
	} from '../attendance-api';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import { attendanceText } from '../text';
	import { getWorkStatusState } from '../work-status/work-status-state.svelte';
	import EmployeeWorkStatusDetailDialog from './employee-work-status-detail-dialog.svelte';
	import { filterEmployeeWorkStatuses } from './employee-work-status-model';
	import EmployeeWorkStatusTable from './employee-work-status-table.svelte';

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();
	const workStatus = getWorkStatusState();
	let search = $state('');
	let statusFilter = $state('all');
	let anchor = $state('');
	let selectedEmployee = $state<AttendanceEmployeeWorkStatus | null>(null);

	const periods: { value: AttendanceWorkStatusPeriod; label: string }[] = [
		{ value: 'day', label: text.day },
		{ value: 'week', label: text.week },
		{ value: 'month', label: text.month }
	];
	const statusOptions = $derived([
		{ value: 'all', label: text.workStatus.allStatuses },
		{ value: 'overtime', label: text.workStatus.statusOvertime },
		{ value: 'remaining', label: text.workStatus.statusRemaining },
		{ value: 'coreTimeMissed', label: text.workStatus.filterCoreTime },
		{ value: 'lateOrEarly', label: text.workStatus.filterLateOrEarly },
		{ value: 'needsReview', label: text.workStatus.statusNeedsReview }
	]);
	const employees = $derived(
		filterEmployeeWorkStatuses(workStatus.payload?.employees ?? [], search, statusFilter)
	);
	const periodLabel = $derived(
		workStatus.payload
			? workStatus.payload.periodStart === workStatus.payload.periodEnd
				? workStatus.payload.periodStart
				: `${workStatus.payload.periodStart}–${workStatus.payload.periodEnd}`
			: anchor
	);

	$effect(() => {
		const summary = attendance.summary;
		if (!summary || anchor) return;
		anchor = todayDateInTimeZone(summary.timeZone);
	});

	function selectPeriod(period: AttendanceWorkStatusPeriod): void {
		attendance.chartMode = period;
		void workStatus.load(period, anchor);
	}

	function movePeriod(direction: -1 | 1): void {
		if (!anchor) return;
		const date = new Date(`${anchor}T00:00:00Z`);
		if (attendance.chartMode === 'month') {
			date.setUTCMonth(date.getUTCMonth() + direction);
		} else {
			date.setUTCDate(date.getUTCDate() + direction * (attendance.chartMode === 'week' ? 7 : 1));
		}
		anchor = date.toISOString().slice(0, 10);
		if (attendance.chartMode === 'month') attendance.selectedMonth = anchor.slice(0, 7);
		void workStatus.load(attendance.chartMode, anchor);
	}

	function statusLabel(status: string): string {
		const labels: Record<string, string> = {
			working: text.workStatus.statusWorking,
			needsReview: text.workStatus.statusNeedsReview,
			coreTimeMissed: text.workStatus.statusCoreTimeMissed,
			late: text.workStatus.statusLate,
			earlyLeave: text.workStatus.statusEarlyLeave,
			lateAndEarlyLeave: text.workStatus.statusLateAndEarlyLeave,
			remaining: text.workStatus.statusRemaining,
			fulfilled: text.workStatus.statusFulfilled,
			overtime: text.workStatus.statusOvertime,
			actualOnly: text.workStatus.statusActualOnly
		};
		return statusOptions.find((option) => option.value === status)?.label ?? labels[status] ?? status;
	}

	function refresh(): void {
		void workStatus.load(attendance.chartMode, anchor);
	}
</script>

<div class="flex min-h-0 min-w-0 flex-col gap-4" data-testid="employee-work-status-view">
	<header class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
		<div>
			<h2 class="text-xl font-semibold">{text.workStatus.employeeTitle}</h2>
			<p class="text-sm text-muted-foreground">{text.workStatus.employeeDescription}</p>
		</div>
		<div class="flex flex-wrap items-center gap-2">
			<div class="flex rounded-md border p-0.5">
				{#each periods as period (period.value)}
					<Button
						variant={attendance.chartMode === period.value ? 'default' : 'ghost'}
						size="sm"
						class="h-7 px-3 text-xs"
						onclick={() => selectPeriod(period.value)}
					>
						{period.label}
					</Button>
				{/each}
			</div>
			<Button variant="outline" size="icon-sm" onclick={() => movePeriod(-1)}>
				<ChevronLeftIcon />
			</Button>
			<span class="min-w-32 text-center text-sm font-medium tabular-nums">{periodLabel}</span>
			<Button variant="outline" size="icon-sm" onclick={() => movePeriod(1)}>
				<ChevronRightIcon />
			</Button>
			<Button variant="ghost" size="icon-sm" onclick={refresh} aria-label={text.refresh}>
				<RefreshCwIcon class={workStatus.isLoading ? 'animate-spin' : ''} />
			</Button>
		</div>
	</header>

	<Card.Root>
		<Card.Header class="gap-3">
			<div class="flex flex-wrap items-center gap-2">
				<div class="flex flex-wrap gap-1">
					{#each statusOptions as option (option.value)}
						<Button
							variant={statusFilter === option.value ? 'secondary' : 'ghost'}
							size="sm"
							onclick={() => (statusFilter = option.value)}
						>
							{option.label}
						</Button>
					{/each}
				</div>
				<Input
					class="ml-auto w-full md:w-64"
					bind:value={search}
					placeholder={text.workStatus.searchPlaceholder}
				/>
			</div>
		</Card.Header>
		<Card.Content class="overflow-x-auto">
			{#if workStatus.errorMessage}
				<p class="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
					{text.workStatus.loadFailed}
				</p>
			{:else}
				<EmployeeWorkStatusTable
					{employees}
					{text}
					{statusLabel}
					onSelect={(employee) => (selectedEmployee = employee)}
				/>
			{/if}
		</Card.Content>
	</Card.Root>
</div>

<EmployeeWorkStatusDetailDialog
	employee={selectedEmployee}
	{text}
	{statusLabel}
	onClose={() => (selectedEmployee = null)}
/>
