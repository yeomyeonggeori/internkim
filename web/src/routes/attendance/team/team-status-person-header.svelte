<script lang="ts">
	import * as HoverCard from '$lib/components/ui/hover-card/index.js';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { mergeProps } from 'bits-ui';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import { formatHoursMinutes } from '../shared/attendance-format';
	import LocationLabel from '../shared/location-label.svelte';
	import WorkTimeChart from '../shared/work-time-chart.svelte';
	import { buildDailyWorkTimeValues, buildWorkTimeChartLocations } from '../shared/work-time-chart-data';
	import type { AttendanceText } from '../text';
	import type { TeamStatusPersonRow } from './team-status-table-model';

	type Props = {
		row: TeamStatusPersonRow;
		columnIndex: number;
		isLastColumn: boolean;
		text: AttendanceText;
		isWorkTimeOpen: boolean;
		onWorkTimeOpenChange: (isOpen: boolean) => void;
	};

	let { row, columnIndex, isLastColumn, text, isWorkTimeOpen, onWorkTimeOpenChange }: Props = $props();

	const attendance = getAttendanceState();
	const dividerClass = $derived(
		`${columnIndex === 0 ? '' : 'border-l'} ${isLastColumn ? 'border-r' : ''}`
	);
	const headerProps = $derived({
		class: `flex min-w-0 flex-col justify-start gap-1 px-2 py-2 ${dividerClass}`,
		role: 'columnheader',
		'data-testid': `team-status-person-header-${row.email}`,
	});
	const employeeEvents = $derived((attendance.summary?.events ?? []).filter((event) => event.email === row.email));
	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));
	const dailyValues = $derived(buildDailyWorkTimeValues(
		attendance.summary?.month ?? '',
		employeeEvents,
		{ currentDate: today, fallbackLocationName: text.location }
	));
	const chartLocations = $derived(buildWorkTimeChartLocations(attendance.summary?.locations ?? [], dailyValues));
</script>

{#snippet PersonHeader({ props }: { props?: Record<string, unknown> })}
	{@const mergedProps = mergeProps(headerProps, props ?? {})}
	<div {...mergedProps}>
		<div class="flex min-w-0 items-center gap-2">
			<PersonAvatar
				name={row.displayName}
				email={row.email}
				seed={row.email || row.mattermostUsername || row.displayName}
				image={row.image ?? ''}
				class="size-7 shrink-0"
			/>
			<div class="min-w-0 flex-1 whitespace-normal break-all text-sm font-medium leading-tight text-foreground">
				{row.displayName}
			</div>
		</div>
		{#if row.currentLocationName}
			<div class="flex min-w-0 justify-end" data-testid="team-status-current-location">
				<LocationLabel name={row.currentLocationName} class="max-w-full" />
			</div>
		{/if}
	</div>
{/snippet}

<HoverCard.Root openDelay={150} bind:open={() => isWorkTimeOpen, onWorkTimeOpenChange}>
	<HoverCard.Trigger>
		{#snippet child({ props })}
			{@render PersonHeader({ props })}
		{/snippet}
	</HoverCard.Trigger>
	<HoverCard.Content side="bottom" align="start" sideOffset={8} class="mx-2 w-[22rem] max-w-[calc(100vw-2rem)] p-0 duration-200 ease-out data-[side=bottom]:slide-in-from-top-1">
		<WorkTimeChart
			title={text.workTime}
			{dailyValues}
			locations={chartLocations}
			formatValue={(value) => formatHoursMinutes(value, text)}
			compact
		/>
	</HoverCard.Content>
</HoverCard.Root>
