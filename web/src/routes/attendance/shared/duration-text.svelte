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
	const showsHours = $derived(hours > 0);
	const showsMinutes = $derived(!showsHours || remainderMinutes > 0);
</script>

<span class={`inline-flex items-baseline gap-1 font-mono tabular-nums ${className}`}>
	{#if showsHours}
		<span>{padDurationNumber(hours)}<span class="font-sans text-[0.8em] font-normal text-muted-foreground">{text.hourUnit}</span></span>
	{/if}
	{#if showsMinutes}
		<span>{padDurationNumber(remainderMinutes)}<span class="font-sans text-[0.8em] font-normal text-muted-foreground">{text.minuteUnit}</span></span>
	{/if}
</span>
