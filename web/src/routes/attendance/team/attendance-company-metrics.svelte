<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { AttendanceTeamPage } from '$lib/attendance/team-page';
	import FlameIcon from '@lucide/svelte/icons/flame';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import PalmtreeIcon from '@lucide/svelte/icons/palmtree';
	import BedIcon from '@lucide/svelte/icons/bed';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { attendanceText } from '../text';

	let {
		summary = null,
		onSelect
	}: {
		summary?: AttendanceTeamPage['companySummary'] | null;
		onSelect?: (status: 'working' | 'done' | 'away' | 'not_started') => void;
	} = $props();
	const text = createPageText(attendanceText);
	const isPhone = new IsMobile();
	const opensStatus = $derived(onSelect !== undefined);
	const metrics = $derived([
		{ status: 'working' as const, label: text.working, icon: FlameIcon, count: summary?.working },
		{ status: 'done' as const, label: text.finished, icon: LogOutIcon, count: summary?.done },
		{ status: 'away' as const, label: text.onLeave, icon: PalmtreeIcon, count: summary?.away },
		{ status: 'not_started' as const, label: text.teamNotStarted, icon: BedIcon, count: summary?.notStarted }
	]);
</script>

{#snippet phoneTile(metric: (typeof metrics)[number])}
	<span class="flex items-center gap-1 whitespace-nowrap text-xs text-muted-foreground"><metric.icon class="size-3.5 shrink-0 max-[359px]:hidden" />{metric.label}</span>
	{#if summary && metric.count !== undefined}
		<span class="text-xl font-semibold leading-7 tabular-nums">{metric.count}</span>
	{:else}
		<Skeleton class="h-7 w-8" aria-hidden="true" />
	{/if}
{/snippet}

{#snippet wideTile(metric: (typeof metrics)[number])}
	<Card.Root class="group-enabled:group-hover:bg-accent/40 h-full gap-2 py-4">
		<Card.Header class="px-4"><Card.Description class="flex items-center gap-1.5"><metric.icon class="size-3.5" />{metric.label}</Card.Description></Card.Header>
		<Card.Content class="flex items-end justify-between px-4">
			{#if summary && metric.count !== undefined}
				<span class="text-3xl font-semibold tracking-tight tabular-nums">{metric.count}<span class="ml-1 text-sm font-normal text-muted-foreground">/ {summary.memberCount}</span></span>
				<span class="text-sm tabular-nums text-muted-foreground">{summary.memberCount ? Math.round(metric.count / summary.memberCount * 100) : 0}%</span>
			{:else}
				<Skeleton class="h-9 w-20" aria-hidden="true" /><Skeleton class="h-5 w-9" aria-hidden="true" />
			{/if}
		</Card.Content>
	</Card.Root>
{/snippet}

{#if isPhone.current}
<div class="grid grid-cols-4 divide-x rounded-lg border" data-testid="attendance-company-summary" aria-busy={!summary}>
	{#each metrics as metric (metric.label)}
		{#if opensStatus}
			<button type="button" class="flex min-w-0 flex-col gap-1 px-3 py-2.5 text-left enabled:hover:bg-accent/50" disabled={!metric.count} onclick={() => onSelect?.(metric.status)} data-testid={`attendance-company-${metric.status}`}>{@render phoneTile(metric)}</button>
		{:else}
			<div class="flex min-w-0 flex-col gap-1 px-3 py-2.5">{@render phoneTile(metric)}</div>
		{/if}
	{/each}
</div>
{:else}
<div class="grid grid-cols-2 gap-3 lg:grid-cols-4" data-testid="attendance-company-summary" aria-busy={!summary}>
	{#each metrics as metric (metric.label)}
		{#if opensStatus}
			<button type="button" class="group text-left disabled:cursor-default" disabled={!metric.count} onclick={() => onSelect?.(metric.status)} data-testid={`attendance-company-${metric.status}`}>{@render wideTile(metric)}</button>
		{:else}
			{@render wideTile(metric)}
		{/if}
	{/each}
</div>
{/if}
