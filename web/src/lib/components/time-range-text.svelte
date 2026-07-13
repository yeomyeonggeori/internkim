<script lang="ts">
	import { formatDisplayTime } from './time-text';
	import { timeTextClasses, type TimeTextSize, type TimeTextTone } from './time-text-variants';

	type Props = {
		startTime: string;
		endTime: string;
		size?: TimeTextSize;
		tone?: TimeTextTone;
		endTone?: TimeTextTone;
	};

	let {
		startTime,
		endTime,
		size = 'small',
		tone = 'muted',
		endTone = 'inherit'
	}: Props = $props();

	const displayedStartTime = $derived(formatDisplayTime(startTime));
	const displayedEndTime = $derived(formatDisplayTime(endTime));
	const label = $derived(`${displayedStartTime}-${displayedEndTime}`);
	const startTimeParts = $derived(splitTime(displayedStartTime));
	const endTimeParts = $derived(splitTime(displayedEndTime));

	function splitTime(value: string): { hours: string; minutes: string } | undefined {
		const match = /^(\d{2}):(\d{2})$/.exec(value);
		if (!match) return undefined;
		const [, hours = '', minutes = ''] = match;
		return { hours, minutes };
	}
</script>

<span
	class={`inline-flex whitespace-nowrap font-mono tabular-nums ${timeTextClasses(size, tone)}`}
	aria-label={label}
	data-slot="time-range-text"
>
	<span data-slot="time-range-start">
		{#if startTimeParts}
			<span>{startTimeParts.hours}</span><span class="opacity-40" aria-hidden="true" data-slot="time-value-separator">:</span><span>{startTimeParts.minutes}</span>
		{:else}
			{displayedStartTime}
		{/if}
	</span>
	<span class="mx-0.5 opacity-40" aria-hidden="true" data-slot="time-range-separator">-</span>
	<span class={timeTextClasses('inherit', endTone)} data-slot="time-range-end">
		{#if endTimeParts}
			<span>{endTimeParts.hours}</span><span class="opacity-40" aria-hidden="true" data-slot="time-value-separator">:</span><span>{endTimeParts.minutes}</span>
		{:else}
			{displayedEndTime}
		{/if}
	</span>
</span>
