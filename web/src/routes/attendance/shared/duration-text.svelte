<script lang="ts">
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { padDurationNumber } from './attendance-format';

	type Props = {
		minutes: number;
		class?: string;
	};

	let { minutes, class: className = '' }: Props = $props();

	const text = createPageText(attendanceText);
	const hours = $derived(Math.floor(minutes / 60));
	const remainderMinutes = $derived(minutes % 60);

	function paddingLength(value: number): number {
		return Math.max(0, padDurationNumber(value).length - String(value).length);
	}

	function durationDigitClass(value: number, index: number): string | undefined {
		if (value === 0 || index < paddingLength(value)) return 'text-muted-foreground/55';
		return undefined;
	}
</script>

{#if minutes > 0}
	<span class={`inline-flex items-baseline gap-[0.35em] font-mono tabular-nums ${className}`}>
		<span class="inline-flex items-baseline">
			{#each Array.from(padDurationNumber(hours)) as character, index}
				<span class={durationDigitClass(hours, index)}>{character}</span>
			{/each}
			<span class="font-sans text-[0.8em] font-normal text-muted-foreground">{text.hourUnit}</span>
		</span>
		<span class="inline-flex items-baseline">
			{#each Array.from(padDurationNumber(remainderMinutes)) as character, index}
				<span class={durationDigitClass(remainderMinutes, index)}>{character}</span>
			{/each}
			<span class="font-sans text-[0.8em] font-normal text-muted-foreground">{text.minuteUnit}</span>
		</span>
	</span>
{/if}
