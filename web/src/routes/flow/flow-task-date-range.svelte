<script lang="ts">
	import { flowTaskDateParts, flowTaskDateText, type FlowTaskDateParts } from './flow-task-date-range';

	type Props = {
		startDate?: string;
		endDate?: string;
		currentYear?: number;
	};

	let { startDate, endDate, currentYear = new Date().getFullYear() }: Props = $props();
	const dates = $derived(
		[startDate, endDate]
			.map((date) => flowTaskDateParts(date, currentYear))
			.filter((date): date is FlowTaskDateParts => date !== undefined)
	);
	const accessibleLabel = $derived(dates.map(flowTaskDateText).join(' - '));
</script>

{#if dates.length}
	<span class="inline-flex items-center font-mono tabular-nums" aria-label={accessibleLabel} data-slot="flow-task-date-range">
		{#each dates as date, index}
			{#if index > 0}
				<span class="mx-1 opacity-40" aria-hidden="true" data-slot="flow-task-date-range-separator">-</span>
			{/if}
			<span class="inline-flex items-center" aria-hidden="true">
				{#if date.year}
					<span>{date.year}</span><span class="opacity-40" data-slot="flow-task-date-separator">/</span>
				{/if}
				<span>{date.month}</span><span class="opacity-40" data-slot="flow-task-date-separator">/</span><span>{date.day}</span>
			</span>
		{/each}
	</span>
{/if}
