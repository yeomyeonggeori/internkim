<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Button } from '$lib/components/ui/button';
	import { companyDateOf } from '$lib/company-time';
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import { supabaseAttendancePersonSummary } from '$lib/attendance/supabase-attendance';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState, type AttendanceSummary } from '../attendance-context.svelte';
	import AttendanceMonthPicker from '../attendance-month-picker.svelte';
	import AttendanceLoadingSkeleton from '../attendance-loading-skeleton.svelte';
	import DeferredSection from '$lib/components/deferred-section.svelte';
	import LeaveBalanceSummary from '../leave/leave-balance-summary.svelte';
	import QuickActions from '../quick-actions.svelte';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import { formatHoursMinutes } from '../shared/attendance-format';
	import WorkTimeChart from '../shared/work-time-chart.svelte';
	import { buildDailyWorkTimeValues, buildWorkTimeChartLocations } from '../shared/work-time-chart-data';
	import { attendanceText } from '../text';
	import WorkStandardSummary from './work-standard-summary.svelte';

	type Props = {
		containerClass?: string;
	};

	let { containerClass = '' }: Props = $props();

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();
	const historyFreshMilliseconds = 30_000;
	const historyCache = new Map<string, { summary: AttendanceSummary; fetchedAt: number }>();
	const pendingHistory = new Map<string, Promise<AttendanceSummary>>();
	let ownMonth = $state('');
	let ownHistory = $state<AttendanceSummary | null>(null);
	let ownHistoryError = $state('');
	let ownHistoryLoading = $state(false);
	let retryRevision = $state(0);
	let historySequence = 0;
	let historyMemberID = '';
	let historyAuthority = '';
	let historyMutation = '';
	let lastRetryRevision = 0;
	let lastCompanyMonth = '';

	$effect(() => {
		const requester = myAttendanceToday.summary;
		const memberID = requester?.currentMemberID;
		const selectedMonth = ownMonth;
		const companyMonth = attendance.summary?.month ?? '';
		const authority = requester
			? `${memberID}|${requester.timeZone}|${requester.isAdmin}|${requester.teamViewVisibleToAll}` : '';
		const mutation = requester
			? `${requester.serverTime ?? ''}|${requester.events.map((event) => `${event.id}:${event.occurredAt}`).join('|')}|${requester.activeLeave?.requestID ?? ''}` : '';
		const askedRetryRevision = retryRevision;
		if (!requester || !memberID) {
			++historySequence;
			ownHistory = null;
			historyMemberID = '';
			historyCache.clear();
			return;
		}
		if (companyMonth !== lastCompanyMonth) {
			lastCompanyMonth = companyMonth;
			if (companyMonth && companyMonth !== selectedMonth) {
				ownMonth = companyMonth;
				return;
			}
		}
		if (historyMemberID !== memberID) {
			++historySequence;
			historyMemberID = memberID;
			historyAuthority = authority;
			historyMutation = mutation;
			historyCache.clear();
			ownMonth = companyDateOf(new Date(requester.serverTime ?? Date.now()), requester.timeZone).slice(0, 7);
			ownHistory = null;
			ownHistoryLoading = false;
			return;
		}
		if (!selectedMonth) return;
		if (historyAuthority !== authority || historyMutation !== mutation || lastRetryRevision !== askedRetryRevision) {
			historyAuthority = authority;
			historyMutation = mutation;
			lastRetryRevision = askedRetryRevision;
			historyCache.clear();
		}
		const request = ++historySequence;
		const key = `${authority}|${mutation}|${selectedMonth}`;
		const cached = historyCache.get(key);
		if (cached && Date.now() - cached.fetchedAt < historyFreshMilliseconds) {
			ownHistory = cached.summary;
			ownHistoryLoading = false;
			ownHistoryError = '';
			return;
		}
		ownHistoryLoading = true;
		ownHistoryError = '';
		let reading = pendingHistory.get(key);
		if (!reading) {
			reading = supabaseAttendancePersonSummary(selectedMonth, {
			memberID, email: requester.currentUserEmail,
			displayName: requester.members.find((person) => person.memberID === memberID)?.displayName ?? requester.currentUserEmail
			}, requester);
			pendingHistory.set(key, reading);
			void reading.then(() => pendingHistory.delete(key), () => pendingHistory.delete(key));
		}
		void reading.then((answer) => {
			if (request !== historySequence) return;
			historyCache.set(key, { summary: answer, fetchedAt: Date.now() });
			ownHistory = answer;
		}).catch((reason) => {
			if (request !== historySequence) return;
			ownHistory = null;
			ownHistoryError = reason instanceof Error ? reason.message : String(reason);
		}).finally(() => {
			if (request === historySequence) ownHistoryLoading = false;
		});
		return () => { ++historySequence; };
	});

	const chartSummary = $derived(
		ownHistory?.month === ownMonth ? ownHistory
			: attendance.summary?.month === ownMonth ? attendance.summary : null
	);
	const targetEmail = $derived(chartSummary?.currentUserEmail || '');
	const chartEvents = $derived(
		(chartSummary?.events ?? []).filter((event) => event.email === targetEmail)
	);
	const today = $derived(todayDateInTimeZone(chartSummary?.timeZone));
	const dailyValues = $derived(buildDailyWorkTimeValues(
		chartSummary?.month ?? '',
		chartEvents,
		{ currentDate: today, fallbackLocationName: text.location, now: new Date(chartSummary?.serverTime ?? Date.now()) }
	));
	const chartLocations = $derived(buildWorkTimeChartLocations(chartSummary?.locations ?? [], dailyValues));
	const recordedTotalMinutes = $derived(dailyValues.reduce((sum, day) =>
		sum + Object.values(day.minutesByLocation).reduce((daySum, minutes) => daySum + minutes, 0), 0));
</script>

<div
	class={`min-h-0 space-y-4 ${containerClass}`}
	data-testid="personal-tools-panel"
>
	{#if !myAttendanceToday.summary}
		<AttendanceLoadingSkeleton rowCount={3} />
	{:else}
		<QuickActions />
		{#if ownMonth}
			<div class="flex items-center justify-end gap-2">
				<AttendanceMonthPicker selectedMonth={ownMonth} onSelectMonth={(month) => (ownMonth = month)} density="compact" />
				{#if ownHistoryLoading}<span class="text-xs text-muted-foreground">{text.loading}</span>{/if}
			</div>
		{/if}
		{#if chartSummary}
			<DeferredSection>
			<WorkTimeChart
				title={text.myWorkTime}
				summary={chartSummary}
				{dailyValues}
				locations={chartLocations}
				formatValue={(value) => formatHoursMinutes(value, text)}
				compact
			>
				{#snippet footer()}
					<div class="mt-2 flex items-center justify-between border-t pt-2 text-xs">
						<span>{text.recorded} {text.total}</span>
						<strong>{formatHoursMinutes(recordedTotalMinutes, text) || `0${text.minuteUnit}`}</strong>
					</div>
					{#if attendance.summary?.month === chartSummary.month}<WorkStandardSummary />{/if}
				{/snippet}
			</WorkTimeChart>
			{#snippet placeholder()}
				<Skeleton class="h-40 w-full rounded-xl" />
			{/snippet}
			</DeferredSection>
		{:else if ownHistoryLoading}
			<Skeleton class="h-40 w-full rounded-xl" />
		{/if}
		{#if ownHistoryError}<p role="alert" class="text-xs text-destructive">{ownHistoryError} <Button variant="link" size="sm" onclick={() => (retryRevision += 1)}>{text.refresh}</Button></p>{/if}
		<DeferredSection>
			<LeaveBalanceSummary />
			{#snippet placeholder()}
				<Skeleton class="h-28 w-full rounded-xl" />
			{/snippet}
		</DeferredSection>
	{/if}
</div>
