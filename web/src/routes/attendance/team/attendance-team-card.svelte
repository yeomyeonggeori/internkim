<script lang="ts" generics="Person extends { name: string }">
	import type { Snippet } from 'svelte';
	import ColorMarker from '$lib/components/color-marker.svelte';
	import * as Card from '$lib/components/ui/card';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { cn } from '$lib/utils';
	import LocationLabel from '../shared/location-label.svelte';
	import { attendanceText } from '../text';
	import type { TeamCardStatus } from './team-card-status';

	type Team = {
		memberCount: number;
		working: number;
		done: number;
		away: number;
		notStarted: number;
		recordedLocations: { name: string; count: number }[];
		unknownLocationCount: number;
		preview: Record<TeamCardStatus, Person[]>;
	};

	let {
		team,
		name,
		isCallerTeam = false,
		face,
		onStatus,
		footer
	}: {
		team: Team;
		name: string;
		isCallerTeam?: boolean;
		face: Snippet<[Person, number]>;
		onStatus?: (status: TeamCardStatus) => void;
		footer?: Snippet;
	} = $props();

	const text = createPageText(attendanceText);
	const isPhone = new IsMobile();
	const namesShownWhole = 2;
	const shownFaces = 3;
	const markerColor: Record<TeamCardStatus, string> = {
		working: 'var(--color-success)',
		done: 'var(--color-muted-foreground)',
		away: 'var(--color-warning)',
		notStarted: 'var(--color-border)'
	};
	const rows = $derived([
		{ status: 'working' as const, label: text.working, count: team.working },
		{ status: 'done' as const, label: text.finished, count: team.done },
		{ status: 'away' as const, label: text.onLeave, count: team.away },
		{ status: 'notStarted' as const, label: text.teamNotStarted, count: team.notStarted }
	]);
	const locationForm = $derived(isPhone.current ? 'marker' : 'tag');
	const opensStatus = $derived(onStatus !== undefined);

	function proportion(count: number, total: number): number {
		return total ? (count / total) * 100 : 0;
	}

	function isDimmed(status: TeamCardStatus): boolean {
		return status === 'done' || status === 'notStarted';
	}

	function namesOf(people: Person[], count: number): { named: string; rest: number } {
		const shown = count <= namesShownWhole ? people.slice(0, namesShownWhole) : people.slice(0, 1);
		return { named: shown.map((person) => displayPersonName(person.name)).join(', '), rest: count - shown.length };
	}
</script>

{#snippet statusRow(row: (typeof rows)[number])}
	{@const people = team.preview[row.status].slice(0, shownFaces)}
	{@const names = namesOf(team.preview[row.status], row.count)}
	<span class="text-muted-foreground flex items-center gap-1.5 text-xs whitespace-nowrap"><ColorMarker color={markerColor[row.status]} />{row.label}</span>
	<span class="flex shrink-0 items-center pl-1.5">
		{#each people as person, index (index)}
			<span class={cn('ring-card relative -ml-1.5 rounded-full ring-2', isDimmed(row.status) && 'grayscale')} style:z-index={people.length - index}>
				{@render face(person, index)}
			</span>
		{/each}
		{#if row.count > people.length}
			<span class="bg-muted text-muted-foreground ring-card relative -ml-2 inline-flex h-6 min-w-6 items-center justify-center rounded-full px-2 text-[11px] tabular-nums ring-2">+{row.count - people.length}</span>
		{/if}
	</span>
	<span class={cn('flex min-w-0 items-baseline gap-1 text-left text-sm', isDimmed(row.status) && 'text-muted-foreground', !row.count && 'text-muted-foreground/60')}>
		{#if row.count}
			<span class="min-w-0 truncate">{names.named}</span>
			{#if names.rest > 0}<span class="text-muted-foreground shrink-0">{text.teamOthers.replace('{count}', String(names.rest))}</span>{/if}
		{:else}—{/if}
	</span>
	<span class={cn('text-right text-sm font-semibold tabular-nums', !row.count && 'text-muted-foreground/60 font-normal')}>{row.count}</span>
{/snippet}

<Card.Root class="gap-0 overflow-hidden rounded-xl py-0" data-testid="attendance-team-card">
	<Card.Header class="gap-3 px-4 pt-4 pb-3">
		<div class="flex items-start justify-between gap-3">
			<div class="min-w-0">
				<Card.Title class="flex min-w-0 items-center gap-1.5 text-base">
					<span class="truncate">{name}</span>
					{#if isCallerTeam}<span class="text-muted-foreground shrink-0 rounded-full border px-1.5 text-[11px] font-normal">{text.teamMine}</span>{/if}
				</Card.Title>
				<p class="text-muted-foreground mt-1 text-xs">{team.memberCount}명</p>
			</div>
			<div class="shrink-0 text-right" aria-label={`${team.working}/${team.memberCount} ${text.working}`}>
				<span class="text-2xl leading-none font-semibold tabular-nums">{Math.round(proportion(team.working, team.memberCount))}%</span>
				<span class="text-muted-foreground block pt-1 text-xs">{text.working} {team.working}/{team.memberCount}</span>
			</div>
		</div>
		<div class="bg-muted flex h-3 overflow-hidden rounded-full max-sm:h-1.5" role="img" aria-label={`${text.working} ${team.working}, ${text.finished} ${team.done}, ${text.onLeave} ${team.away}, ${text.teamNotStarted} ${team.notStarted}`}>
			<span style:background-color={markerColor.working} style:width={`${proportion(team.working, team.memberCount)}%`}></span>
			<span style:background-color={markerColor.done} style:width={`${proportion(team.done, team.memberCount)}%`}></span>
			<span style:background-color={markerColor.away} style:width={`${proportion(team.away, team.memberCount)}%`}></span>
		</div>
	</Card.Header>
	<Card.Content class="flex flex-col px-2 pb-2">
		{#each rows as row (row.status)}
			{#if opensStatus}
				<button
					type="button"
					class="hover:bg-accent/50 grid h-10 grid-cols-[4.5rem_auto_minmax(0,1fr)_1.5rem] items-center gap-2.5 rounded-lg px-2 disabled:pointer-events-none"
					disabled={!row.count}
					onclick={() => onStatus?.(row.status)}
					data-testid={`attendance-team-status-${row.status}`}
				>
					{@render statusRow(row)}
				</button>
			{:else}
				<div class="grid h-10 grid-cols-[4.5rem_auto_minmax(0,1fr)_1.5rem] items-center gap-2.5 px-2">{@render statusRow(row)}</div>
			{/if}
		{/each}
		<div class="text-muted-foreground border-border/70 mx-2 mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 border-t pt-3 pb-1 text-xs">
			<span>{text.teamRecordedLocations}</span>
			{#each team.recordedLocations as location (location.name)}
				<LocationLabel name={location.name} count={location.count} form={locationForm} />
			{/each}
			{#if team.unknownLocationCount}<LocationLabel name={text.teamUnknownLocation} count={team.unknownLocationCount} form={locationForm} />{/if}
			{#if !team.recordedLocations.length && !team.unknownLocationCount}<span>—</span>{/if}
		</div>
	</Card.Content>
	{#if footer}
		<Card.Footer class="bg-card border-border/70 border-t p-0">{@render footer()}</Card.Footer>
	{/if}
</Card.Root>
