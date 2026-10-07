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

	let { summary = null }: { summary?: AttendanceTeamPage['companySummary'] | null } = $props();
	const text = createPageText(attendanceText);
	const isPhone = new IsMobile();
	const metrics = $derived([
		{ label: text.working, icon: FlameIcon, count: summary?.working },
		{ label: text.finished, icon: LogOutIcon, count: summary?.done },
		{ label: text.onLeave, icon: PalmtreeIcon, count: summary?.away },
		{ label: text.teamNotStarted, icon: BedIcon, count: summary?.notStarted }
	]);
</script>

{#if isPhone.current}
<div class="grid grid-cols-4 divide-x rounded-lg border" data-testid="attendance-company-summary" aria-busy={!summary}>
	{#each metrics as metric (metric.label)}
		<div class="flex min-w-0 flex-col gap-1 px-3 py-2.5">
			<span class="flex items-center gap-1 whitespace-nowrap text-xs text-muted-foreground"><metric.icon class="size-3.5 shrink-0 max-[359px]:hidden" />{metric.label}</span>
			{#if summary && metric.count !== undefined}
				<span class="text-xl font-semibold leading-7 tabular-nums">{metric.count}</span>
			{:else}
				<Skeleton class="h-7 w-8" aria-hidden="true" />
			{/if}
		</div>
	{/each}
</div>
{:else}
<div class="grid grid-cols-2 gap-3 lg:grid-cols-4" data-testid="attendance-company-summary" aria-busy={!summary}>
	{#each metrics as metric (metric.label)}
		<Card.Root class="gap-2 py-4">
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
	{/each}
</div>
{/if}
