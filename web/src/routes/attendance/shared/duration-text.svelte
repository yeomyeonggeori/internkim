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
</script>

{#if minutes > 0}
	<span class={`inline-flex items-baseline gap-1 font-mono tabular-nums ${className}`}>
		<span>
			{#each Array.from(padDurationNumber(hours)) as character, index}
				<span class={index < paddingLength(hours) ? 'text-muted-foreground/55' : undefined}>{character}</span>
			{/each}
			<span class="font-sans text-[0.8em] font-normal text-muted-foreground">{text.hourUnit}</span>
		</span>
		<span>
			{#each Array.from(padDurationNumber(remainderMinutes)) as character, index}
				<span class={index < paddingLength(remainderMinutes) ? 'text-muted-foreground/55' : undefined}>{character}</span>
			{/each}
			<span class="font-sans text-[0.8em] font-normal text-muted-foreground">{text.minuteUnit}</span>
		</span>
	</span>
{/if}
