<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { getAttendanceState, type AttendanceEvent } from '../attendance-context.svelte';
	import { computePersonalStats } from '../shared/attendance-aggregation';
	import { eachDayOfMonth } from '../shared/attendance-date';
	import { computeDayEvents } from '../shared/attendance-day-events';
	import { attendanceText } from '../text';

	type Props = {
		compact?: boolean;
	};

	let { compact = false }: Props = $props();
	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);

	const targetEmail = $derived(attendance.summary?.currentUserEmail || '');
	const events = $derived(
		attendance.summary ? attendance.summary.events.filter((e) => e.email === targetEmail) : []
	);
	const stats = $derived(
		attendance.summary ? computePersonalStats(attendance.summary.month, events) : null
	);
	const locationBreakdown = $derived(buildLocationBreakdown(attendance.summary?.month ?? '', events));
	const ringDeg = $derived(stats && stats.weekdayCount ? Math.round((stats.workedDays / stats.weekdayCount) * 360) : 0);

	function buildLocationBreakdown(month: string, eventList: AttendanceEvent[]) {
		const count = new Map<string, number>();
		let total = 0;
		for (const date of eachDayOfMonth(month)) {
			const day = computeDayEvents(date, eventList);
			for (const segment of day.segments) {
				const label = segment.locationName ?? '-';
				count.set(label, (count.get(label) ?? 0) + 1);
				total += 1;
			}
		}
		return [...count.entries()]
			.sort((a, b) => b[1] - a[1])
			.map(([label, n]) => ({ label, count: n, percent: total ? Math.round((n / total) * 100) : 0 }));
	}

	function formatSegmentCount(count: number): string {
		return text.locationSegmentCountTemplate.replace('{count}', String(count));
	}
</script>

<div class={compact ? 'flex flex-col gap-2' : 'flex flex-col gap-3'}>
	{#if stats}
		<Card.Root>
			<Card.Header class={compact ? 'pb-2' : undefined}><Card.Title class="text-sm">{text.attendanceRate}</Card.Title></Card.Header>
			<Card.Content class="flex flex-col items-center">
				<div
					class={compact ? 'flex h-16 w-16 items-center justify-center rounded-full' : 'flex h-20 w-20 items-center justify-center rounded-full'}
					style={`background: conic-gradient(#22c55e 0deg ${ringDeg}deg, #e5e7eb ${ringDeg}deg 360deg)`}
				>
					<div class={compact ? 'flex h-11 w-11 flex-col items-center justify-center rounded-full bg-background' : 'flex h-14 w-14 flex-col items-center justify-center rounded-full bg-background'}>
						<span class="text-sm font-semibold">{stats.weekdayCount ? Math.round((stats.workedDays / stats.weekdayCount) * 100) : 0}%</span>
						<span class="text-[9px] text-muted-foreground">{stats.workedDays}/{stats.weekdayCount}</span>
					</div>
				</div>
			</Card.Content>
		</Card.Root>
	{/if}

	<Card.Root>
		<Card.Header class={compact ? 'pb-2' : undefined}><Card.Title class="text-sm">{text.locationDistribution}</Card.Title></Card.Header>
		<Card.Content class={compact ? 'space-y-1 px-3 pb-3 text-xs' : 'space-y-1 text-xs'}>
			{#each locationBreakdown as item (item.label)}
				<div class="flex items-center justify-between">
					<span>{item.label}</span>
					<span class="text-muted-foreground">{formatSegmentCount(item.count)} ({item.percent}%)</span>
				</div>
			{/each}
			{#if locationBreakdown.length === 0}
				<p class="text-muted-foreground">{text.noData}</p>
			{/if}
		</Card.Content>
	</Card.Root>
</div>
