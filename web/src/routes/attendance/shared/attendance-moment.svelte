<script lang="ts">
	import BedIcon from '@lucide/svelte/icons/bed';
	import LogInIcon from '@lucide/svelte/icons/log-in';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import PalmtreeIcon from '@lucide/svelte/icons/palmtree';
	import { companyDateOf } from '$lib/company-time';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import LocationLabel from './location-label.svelte';

	let {
		status,
		latestAt,
		location,
		timeZone
	}: {
		status: 'working' | 'done' | 'away' | 'needs_checkout' | 'not_started';
		latestAt: string | null;
		location: string | null;
		timeZone: string;
	} = $props();

	const text = createPageText(attendanceText);
	const isAtWork = $derived(status === 'working' || status === 'needs_checkout');

	function momentOf(instant: string): string {
		const zone = timeZone || 'UTC';
		const isToday = companyDateOf(new Date(instant), zone) === companyDateOf(new Date(), zone);
		return new Intl.DateTimeFormat(text.dateLocale, {
			...(isToday ? {} : { month: 'numeric', day: 'numeric' }),
			hour: '2-digit',
			minute: '2-digit',
			timeZone: zone
		}).format(new Date(instant));
	}
</script>

<span class="text-muted-foreground flex min-w-0 items-center gap-1.5 text-xs whitespace-nowrap">
	{#if isAtWork}
		{#if location}<LocationLabel name={location} />{/if}
		{#if latestAt}<span class="text-foreground inline-flex items-center gap-1 tabular-nums" aria-label={`${text.clockIn} ${momentOf(latestAt)}`}><LogInIcon class="text-muted-foreground size-3.5" />{momentOf(latestAt)}</span>{/if}
	{:else if status === 'done'}
		{#if latestAt}<span class="inline-flex items-center gap-1 tabular-nums" aria-label={`${text.clockOut} ${momentOf(latestAt)}`}><LogOutIcon class="size-3.5" />{momentOf(latestAt)}</span>{/if}
	{:else if status === 'away'}
		<span class="inline-flex items-center gap-1"><PalmtreeIcon class="size-3.5" />{text.onLeave}</span>
	{:else}
		<span class="inline-flex items-center gap-1"><BedIcon class="size-3.5" />{text.teamNotStarted}</span>
	{/if}
</span>
