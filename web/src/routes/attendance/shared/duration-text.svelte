<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { timeTextClasses, type TimeTextSize, type TimeTextTone } from '$lib/components/time-text-variants';
	import { attendanceText } from '../text';
	import { padDurationNumber } from './attendance-format';

	type Props = {
		minutes: number;
		showZero?: boolean;
		size?: TimeTextSize;
		tone?: TimeTextTone;
	};

	let {
		minutes,
		showZero = false,
		size = 'small',
		tone = 'inherit'
	}: Props = $props();

	const text = createPageText(attendanceText);
	const hours = $derived(Math.floor(minutes / 60));
	const remainderMinutes = $derived(minutes % 60);
	const durationLabel = $derived(`${padDurationNumber(hours)}${text.hourUnit} ${padDurationNumber(remainderMinutes)}${text.minuteUnit}`);

	function paddingLength(value: number): number {
		return Math.max(0, padDurationNumber(value).length - String(value).length);
	}

	function durationDigitClass(value: number, index: number): string | undefined {
		if (value === 0 || index < paddingLength(value)) return 'opacity-40';
		return undefined;
	}
</script>

{#if minutes > 0 || showZero}
	<span
		class={`inline-flex items-baseline gap-[0.35em] whitespace-nowrap font-mono font-medium tabular-nums ${timeTextClasses(size, tone)}`}
		aria-label={durationLabel}
		data-slot="duration-text"
	>
		<span class="inline-flex items-baseline gap-0.5">
			{#each Array.from(padDurationNumber(hours)) as character, index}
				<span class={durationDigitClass(hours, index)} data-slot="duration-digit">{character}</span>
			{/each}
			<span class="font-sans text-[0.8em] font-normal text-current opacity-60" data-slot="duration-unit">{text.hourUnit}</span>
		</span>
		<span class="inline-flex items-baseline gap-0.5">
			{#each Array.from(padDurationNumber(remainderMinutes)) as character, index}
				<span class={durationDigitClass(remainderMinutes, index)} data-slot="duration-digit">{character}</span>
			{/each}
			<span class="font-sans text-[0.8em] font-normal text-current opacity-60" data-slot="duration-unit">{text.minuteUnit}</span>
		</span>
	</span>
{/if}
