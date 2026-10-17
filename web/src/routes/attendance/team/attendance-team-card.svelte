<script lang="ts" generics="Actor">
	import type { Snippet } from 'svelte';
	import ColorMarker from '$lib/components/color-marker.svelte';
	import * as Card from '$lib/components/ui/card';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import LocationLabel from '../shared/location-label.svelte';
	import { attendanceText } from '../text';

	type Team = {
		memberCount: number;
		working: number;
		done: number;
		away: number;
		notStarted: number;
		recordedLocations: { name: string; count: number }[];
		unknownLocationCount: number;
		recentClockIns: Actor[];
		recentClockOuts: Actor[];
	};

	let {
		team,
		name,
		actorStack,
		footer
	}: { team: Team; name: string; actorStack: Snippet<[Actor[], string]>; footer?: Snippet } = $props();

	const text = createPageText(attendanceText);
	const isPhone = new IsMobile();
	const workingColor = $derived(isPhone.current ? 'color-mix(in oklch, var(--color-success) 45%, var(--color-muted-foreground))' : 'var(--color-success)');
	const awayColor = $derived(isPhone.current ? 'color-mix(in oklch, var(--color-warning) 70%, var(--color-muted-foreground))' : 'var(--color-warning)');
	const locationForm = $derived(isPhone.current ? 'marker' : 'tag');

	function proportion(count: number, total: number): number {
		return total ? count / total * 100 : 0;
	}
</script>

<Card.Root class="gap-0 overflow-hidden rounded-xl py-0" data-testid="attendance-team-card">
	<Card.Header class="gap-3 px-4 pb-3 pt-4">
		<div class="flex items-start justify-between gap-3">
			<div class="min-w-0"><Card.Title class="truncate text-base">{name}</Card.Title><p class="mt-1 text-xs text-muted-foreground">{team.memberCount}명</p></div>
			<div class="shrink-0 text-right" aria-label={`${team.working}/${team.memberCount} ${text.working}`}>
				<span class="text-2xl font-semibold leading-none tabular-nums">{Math.round(proportion(team.working, team.memberCount))}%</span>
				<span class="block pt-1 text-xs text-muted-foreground">{text.working} {team.working}/{team.memberCount}</span>
			</div>
		</div>
		<div class="flex h-3 overflow-hidden rounded-full bg-muted max-sm:h-1.5" role="img" aria-label={`${text.working} ${team.working}, ${text.finished} ${team.done}, ${text.onLeave} ${team.away}, ${text.teamNotStarted} ${team.notStarted}`}>
			<span style:background-color={workingColor} style:width={`${proportion(team.working, team.memberCount)}%`}></span>
			<span style:background-color="var(--color-muted-foreground)" style:width={`${proportion(team.done, team.memberCount)}%`}></span>
			<span style:background-color={awayColor} style:width={`${proportion(team.away, team.memberCount)}%`}></span>
		</div>
	</Card.Header>
	<Card.Content class="flex flex-col gap-4 px-4 pb-4">
		<div class="grid grid-cols-2 gap-x-6 gap-y-2 text-xs text-muted-foreground">
			<span class="flex items-center justify-between gap-4"><span class="inline-flex items-center gap-1.5"><ColorMarker color={workingColor} /><span>{text.working}</span></span><strong class="shrink-0 tabular-nums text-foreground">{team.working}</strong></span>
			<span class="flex items-center justify-between gap-4"><span class="inline-flex items-center gap-1.5"><ColorMarker color="var(--color-muted-foreground)" /><span>{text.finished}</span></span><strong class="shrink-0 tabular-nums text-foreground">{team.done}</strong></span>
			<span class="flex items-center justify-between gap-4"><span class="inline-flex items-center gap-1.5"><ColorMarker color={awayColor} /><span>{text.onLeave}</span></span><strong class="shrink-0 tabular-nums text-foreground">{team.away}</strong></span>
			<span class="flex items-center justify-between gap-4"><span class="inline-flex items-center gap-1.5"><ColorMarker color="var(--color-muted)" /><span>{text.teamNotStarted}</span></span><strong class="shrink-0 tabular-nums text-foreground">{team.notStarted}</strong></span>
		</div>
		<div class="flex flex-wrap items-center gap-x-2 gap-y-1 border-t border-border/70 pt-3 text-xs text-muted-foreground">
			<span>{text.teamRecordedLocations}</span>
			{#each team.recordedLocations as location (location.name)}
				<LocationLabel name={location.name} count={location.count} form={locationForm} />
			{/each}
			{#if team.unknownLocationCount}<LocationLabel name={text.teamUnknownLocation} count={team.unknownLocationCount} form={locationForm} />{/if}
			{#if !team.recordedLocations.length && !team.unknownLocationCount}<span>—</span>{/if}
		</div>
		<div class="flex flex-col gap-3 border-t border-border/70 pt-3 max-sm:grid max-sm:grid-cols-2 max-sm:gap-4">
			{@render actorStack(team.recentClockIns, text.teamRecentIns)}
			{@render actorStack(team.recentClockOuts, text.teamRecentOuts)}
		</div>
	</Card.Content>
	{#if footer}
		<Card.Footer class="border-t border-border/70 bg-card p-0">{@render footer()}</Card.Footer>
	{/if}
</Card.Root>
