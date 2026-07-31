<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import * as Table from '$lib/components/ui/table';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
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
	import { formatWorkStatusDuration } from '../work-status/work-status-format';
	import { filterEmployeeWorkStatuses } from './employee-work-status-model';

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
		{ value: 'remaining', label: text.workStatus.statusRemaining },
		{ value: 'fulfilled', label: text.workStatus.statusFulfilled },
		{ value: 'overtime', label: text.workStatus.statusOvertime },
		{ value: 'coreTimeMissed', label: text.workStatus.statusCoreTimeMissed },
		{ value: 'late', label: text.workStatus.statusLate },
		{ value: 'earlyLeave', label: text.workStatus.statusEarlyLeave },
		{ value: 'needsReview', label: text.workStatus.statusNeedsReview },
		{ value: 'actualOnly', label: text.workStatus.statusActualOnly }
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
			lateAndEarlyLeave: text.workStatus.statusLateAndEarlyLeave
		};
		return statusOptions.find((option) => option.value === status)?.label ?? labels[status] ?? status;
	}

	function formatDifference(minutes: number): string {
		const prefix = minutes > 0 ? '+' : minutes < 0 ? '-' : '';
		return `${prefix}${formatWorkStatusDuration(Math.abs(minutes), text)}`;
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
			<div class="grid gap-2 md:grid-cols-[minmax(15rem,1fr)_auto]">
				<Input bind:value={search} placeholder={text.workStatus.searchPlaceholder} />
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
			</div>
		</Card.Header>
		<Card.Content class="overflow-x-auto">
			{#if workStatus.errorMessage}
				<p class="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
					{text.workStatus.loadFailed}
				</p>
			{:else}
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>{text.workStatus.employee}</Table.Head>
							<Table.Head>{text.workStatus.mode}</Table.Head>
							<Table.Head>{text.status}</Table.Head>
							<Table.Head class="text-right">{text.workStatus.actual}</Table.Head>
							<Table.Head class="text-right">{text.workStatus.paidLeave}</Table.Head>
							<Table.Head class="text-right">{text.workStatus.fulfilled}</Table.Head>
							<Table.Head class="text-right">{text.workStatus.target}</Table.Head>
							<Table.Head class="text-right">{text.workStatus.difference}</Table.Head>
							<Table.Head class="text-right">{text.workStatus.overtime}</Table.Head>
							<Table.Head class="text-right">{text.workStatus.night}</Table.Head>
							<Table.Head class="text-right">{text.workStatus.details}</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each employees as employee (employee.email)}
							<Table.Row>
								<Table.Cell>
									<div class="flex items-center gap-2">
										<PersonAvatar
											name={employee.displayName}
											email={employee.email}
											seed={employee.email}
											class="size-8 shrink-0"
										/>
										<span class="grid">
											<span class="font-medium">{employee.displayName}</span>
											<span class="text-xs text-muted-foreground">{employee.email}</span>
										</span>
									</div>
								</Table.Cell>
								<Table.Cell>{text.workStatus[employee.workMode]}</Table.Cell>
								<Table.Cell><Badge variant="secondary">{statusLabel(employee.status)}</Badge></Table.Cell>
								<Table.Cell class="text-right tabular-nums">{formatWorkStatusDuration(employee.actualMinutes, text)}</Table.Cell>
								<Table.Cell class="text-right tabular-nums">{formatWorkStatusDuration(employee.paidLeaveMinutes, text)}</Table.Cell>
								<Table.Cell class="text-right tabular-nums">{formatWorkStatusDuration(employee.fulfilledMinutes, text)}</Table.Cell>
								<Table.Cell class="text-right tabular-nums">{formatWorkStatusDuration(employee.targetMinutes, text)}</Table.Cell>
								<Table.Cell class="text-right tabular-nums">{formatDifference(employee.differenceMinutes)}</Table.Cell>
								<Table.Cell class="text-right tabular-nums">{formatWorkStatusDuration(employee.overtimeMinutes, text)}</Table.Cell>
								<Table.Cell class="text-right tabular-nums">{formatWorkStatusDuration(employee.nightMinutes, text)}</Table.Cell>
								<Table.Cell class="text-right">
									<Button variant="outline" size="sm" onclick={() => (selectedEmployee = employee)}>
										{text.workStatus.details}
									</Button>
								</Table.Cell>
							</Table.Row>
						{:else}
							<Table.Row>
								<Table.Cell colspan={11} class="h-28 text-center text-muted-foreground">
									{text.workStatus.noEmployees}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			{/if}
		</Card.Content>
	</Card.Root>
</div>

<Dialog.Root
	open={selectedEmployee !== null}
	onOpenChange={(open) => {
		if (!open) selectedEmployee = null;
	}}
>
	<Dialog.Content class="max-h-[85vh] overflow-y-auto sm:max-w-3xl" closeLabel={text.close}>
		{#if selectedEmployee}
			<Dialog.Header>
				<Dialog.Title>{selectedEmployee.displayName} · {text.workStatus.dailyBreakdown}</Dialog.Title>
				<Dialog.Description>
					{selectedEmployee.periodStart}–{selectedEmployee.periodEnd}
				</Dialog.Description>
			</Dialog.Header>
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>{text.date}</Table.Head>
						<Table.Head>{text.status}</Table.Head>
						<Table.Head class="text-right">{text.workStatus.target}</Table.Head>
						<Table.Head class="text-right">{text.workStatus.actual}</Table.Head>
						<Table.Head class="text-right">{text.workStatus.paidLeave}</Table.Head>
						<Table.Head class="text-right">{text.workStatus.overtime}</Table.Head>
						<Table.Head class="text-right">{text.workStatus.night}</Table.Head>
						<Table.Head>{text.workStatus.workSegments}</Table.Head>
						<Table.Head>{text.workStatus.leaveSegments}</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each selectedEmployee.days as day (day.date)}
						<Table.Row>
							<Table.Cell>{day.date}</Table.Cell>
							<Table.Cell><Badge variant="secondary">{statusLabel(day.status)}</Badge></Table.Cell>
							<Table.Cell class="text-right">{formatWorkStatusDuration(day.targetMinutes, text)}</Table.Cell>
							<Table.Cell class="text-right">{formatWorkStatusDuration(day.actualMinutes, text)}</Table.Cell>
							<Table.Cell class="text-right">{formatWorkStatusDuration(day.paidLeaveMinutes, text)}</Table.Cell>
							<Table.Cell class="text-right">{formatWorkStatusDuration(day.overtimeMinutes, text)}</Table.Cell>
							<Table.Cell class="text-right">{formatWorkStatusDuration(day.nightMinutes, text)}</Table.Cell>
							<Table.Cell>
								{#if day.workSegments.length > 0}
									{day.workSegments
										.map((segment) => `${segment.startTime}–${segment.endTime}${segment.provisional ? ` (${text.workStatus.statusWorking})` : ''}`)
										.join(', ')}
								{:else}
									{text.workStatus.noSegments}
								{/if}
							</Table.Cell>
							<Table.Cell>
								{#if day.leaveSegments.length > 0}
									{day.leaveSegments
										.map((segment) => `${segment.startTime}–${segment.endTime} (${segment.paid ? text.workStatus.paid : text.workStatus.unpaid})`)
										.join(', ')}
								{:else}
									{text.workStatus.noSegments}
								{/if}
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
			<Dialog.Footer>
				<Button variant="outline" onclick={() => (selectedEmployee = null)}>
					{text.workStatus.close}
				</Button>
			</Dialog.Footer>
		{/if}
	</Dialog.Content>
</Dialog.Root>
