<script lang="ts">
	import { onMount } from 'svelte';
	import SearchIcon from '@lucide/svelte/icons/search';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
	import Clock3Icon from '@lucide/svelte/icons/clock-3';
	import MapPinIcon from '@lucide/svelte/icons/map-pin';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as InputGroup from '$lib/components/ui/input-group';
	import * as Select from '$lib/components/ui/select';
	import * as Sheet from '$lib/components/ui/sheet';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import type { AttendanceTeamPage } from '$lib/attendance/team-page';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { getAttendanceTeamState } from './attendance-team-state.svelte';

	let { onOpenMonthly }: { onOpenMonthly: (member?: AttendanceTeamPage['members'][number]) => void } = $props();
	const text = createPageText(attendanceText);
	const teamState = getAttendanceTeamState();
	let searchDraft = $state('');
	let locationDraft = $state('');
	let selectedMember = $state<AttendanceTeamPage['members'][number] | null>(null);
	let sheetOpen = $state(false);
	const selectedTeam = $derived(teamState.teams.find((team) => team.teamKey === teamState.selectedTeamKey));
	const knownLocations = $derived(myAttendanceToday.summary?.locations ?? []);

	onMount(() => teamState.start());

	$effect(() => {
		if (!teamState.selectedTeamKey) return;
		const search = searchDraft.trim();
		const location = locationDraft;
		if (search === teamState.searchText && location === teamState.locationFilter) return;
		const timer = setTimeout(() => teamState.filter(search, location), 250);
		return () => clearTimeout(timer);
	});

	function timeOf(instant: string): string {
		return new Intl.DateTimeFormat(text.dateLocale, {
			hour: '2-digit', minute: '2-digit', timeZone: teamState.timeZone || 'UTC'
		}).format(new Date(instant));
	}

	function statusName(status: string): string {
		if (status === 'working') return text.working;
		if (status === 'done') return text.finished;
		if (status === 'away') return text.onLeave;
		if (status === 'needs_checkout') return text.teamNeedsCheckout;
		return text.teamNotStarted;
	}

	function proportion(count: number, total: number): number {
		return total ? count / total * 100 : 0;
	}

	function selectTeam(key: string): void {
		searchDraft = '';
		locationDraft = '';
		teamState.selectTeam(key);
	}

	function backToTeams(): void {
		teamState.clearSelection();
		searchDraft = '';
		locationDraft = '';
	}

	type RecentActor = AttendanceTeamPage['teams'][number]['recentClockIns'][number];
</script>

{#snippet actorStack(actors: RecentActor[], label: string)}
	<div class="flex min-w-0 items-center justify-between gap-2">
		<span class="text-xs text-muted-foreground">{label}</span>
		{#if actors.length}
			<div class="flex -space-x-2" aria-label={label}>
				{#each actors as actor (actor.memberID + actor.occurredAt)}
					<Tooltip.Root>
						<Tooltip.Trigger aria-label={actor.name + ' ' + timeOf(actor.occurredAt)} class="relative rounded-full ring-2 ring-card">
							<PersonAvatar name={actor.name} email={actor.email} class="size-8" />
						</Tooltip.Trigger>
						<Tooltip.Content>{actor.name} · {timeOf(actor.occurredAt)}</Tooltip.Content>
					</Tooltip.Root>
				{/each}
			</div>
		{:else}
			<span class="text-xs text-muted-foreground">—</span>
		{/if}
	</div>
{/snippet}

<Tooltip.Provider>
	<div class="mx-auto flex w-full max-w-7xl flex-col gap-5" data-testid="attendance-team-dashboard">
		<div class="flex flex-wrap items-end justify-between gap-3">
			<div>
				{#if teamState.selectedTeamKey}
					<Button variant="ghost" size="sm" class="-ml-2 mb-1" onclick={backToTeams}>
						<ArrowLeftIcon />{text.teamBack}
					</Button>
				{/if}
				<h2 class="text-xl font-semibold tracking-tight">{selectedTeam?.name === 'Unassigned' ? text.teamUnassigned : selectedTeam?.name ?? text.teamTodayTitle}</h2>
			</div>
			<Button variant="outline" size="sm" onclick={() => onOpenMonthly()}>{text.teamMonthlyOpen}</Button>
		</div>

		{#if !teamState.selectedTeamKey}
			{#if teamState.error}
				<p role="alert" class="text-sm text-destructive">{text.teamLoadFailed} {teamState.error}</p>
				<Button variant="outline" size="sm" onclick={() => teamState.loadTeams()}>{text.refresh}</Button>
			{:else if teamState.isLoadingTeams && !teamState.teams.length}
				<p class="text-sm text-muted-foreground" data-testid="team-cards-loading">{text.teamTodayTitle}…</p>
			{:else}
				<div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3" data-testid="team-card-page">
					{#each teamState.teams as team (team.teamKey)}
						<Card.Root class="gap-0 overflow-hidden rounded-xl py-0" data-testid="attendance-team-card">
							<Card.Header class="gap-3 px-4 pb-3 pt-4">
								<div class="flex items-start justify-between gap-3">
									<Card.Title class="min-w-0 truncate text-base">{team.name === 'Unassigned' ? text.teamUnassigned : team.name}</Card.Title>
									<div class="shrink-0 text-right" aria-label={`${team.working}/${team.memberCount} ${text.working}`}>
										<span class="text-2xl font-semibold leading-none tabular-nums">{team.working}<span class="text-sm font-normal text-muted-foreground">/{team.memberCount}</span></span>
										<span class="block pt-1 text-xs text-muted-foreground">{text.working}</span>
									</div>
								</div>
								<div class="flex h-1.5 overflow-hidden rounded-full bg-muted" role="img" aria-label={`${text.working} ${team.working}, ${text.finished} ${team.done}, ${text.onLeave} ${team.away}, ${text.teamNeedsCheckout} ${team.needsCheckout}, ${text.teamNotStarted} ${team.notStarted}`}>
									<span class="bg-emerald-500" style:width={`${proportion(team.working, team.memberCount)}%`}></span>
									<span class="bg-slate-400" style:width={`${proportion(team.done, team.memberCount)}%`}></span>
									<span class="bg-amber-400" style:width={`${proportion(team.away, team.memberCount)}%`}></span>
									<span class="bg-rose-400" style:width={`${proportion(team.needsCheckout, team.memberCount)}%`}></span>
								</div>
							</Card.Header>
							<Card.Content class="space-y-3 px-4 pb-3">
								<div class="flex min-h-4 flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
									{#if team.done}<span>{text.finished} {team.done}</span>{/if}
									{#if team.away}<span>{text.onLeave} {team.away}</span>{/if}
									{#if team.needsCheckout}<span class="text-rose-600 dark:text-rose-400">{text.teamNeedsCheckout} {team.needsCheckout}</span>{/if}
									{#if team.notStarted}<span>{text.teamNotStarted} {team.notStarted}</span>{/if}
								</div>
								<div class="space-y-2 border-t border-border/70 pt-3">
									{@render actorStack(team.recentClockIns, text.teamRecentIns)}
									{@render actorStack(team.recentClockOuts, text.teamRecentOuts)}
								</div>
								<div class="flex flex-wrap items-center gap-x-2 gap-y-1 border-t border-border/70 pt-3 text-xs text-muted-foreground">
									<MapPinIcon class="size-3.5 shrink-0" />
									<span>{text.teamRecordedLocations}</span>
									{#each team.recordedLocations as location (location.name)}
										<span class="font-medium text-foreground">{location.name} · {location.count}</span>
									{/each}
									{#if team.unknownLocationCount}<span>{text.teamUnknownLocation} · {team.unknownLocationCount}</span>{/if}
									{#if !team.recordedLocations.length && !team.unknownLocationCount}<span>—</span>{/if}
								</div>
							</Card.Content>
							<Card.Footer class="border-t border-border/70 bg-card p-0">
								<Button variant="ghost" class="h-11 w-full justify-between rounded-none px-4 text-sm font-medium" onclick={() => selectTeam(team.teamKey)}>
								{text.teamEmployees}<ArrowRightIcon class="size-4" />
							</Button>
						</Card.Footer>
						</Card.Root>
					{/each}
				</div>
				<div class="flex items-center justify-between gap-3 text-sm text-muted-foreground">
					<span>{teamState.teamOffset + 1}–{Math.min(teamState.teamOffset + 12, teamState.teamTotal)} / {teamState.teamTotal}</span>
					<div class="flex gap-2">
						<Button variant="outline" size="sm" disabled={teamState.teamOffset === 0 || teamState.isLoadingTeams} onclick={() => teamState.loadTeams(Math.max(0, teamState.teamOffset - 12))}>{text.teamPagePrevious}</Button>
						<Button variant="outline" size="sm" disabled={teamState.teamOffset + 12 >= teamState.teamTotal || teamState.isLoadingTeams} onclick={() => teamState.loadTeams(teamState.teamOffset + 12)}>{text.teamPageNext}</Button>
					</div>
				</div>
			{/if}
		{:else}
			<div class="flex flex-col gap-3 sm:flex-row">
				<InputGroup.Root class="min-w-0 flex-1">
					<InputGroup.Addon align="inline-start"><SearchIcon /></InputGroup.Addon>
					<InputGroup.Input aria-label={text.teamSearch} placeholder={text.teamSearch} bind:value={searchDraft} />
				</InputGroup.Root>
				<Select.Root type="single" value={locationDraft || '__all'} onValueChange={(value) => (locationDraft = value === '__all' ? '' : value)}>
					<Select.Trigger class="w-full sm:w-52">{locationDraft || text.allLocations}</Select.Trigger>
					<Select.Content>
						<Select.Item value="__all" label={text.allLocations}>{text.allLocations}</Select.Item>
						{#each knownLocations as location (location.name)}
							<Select.Item value={location.name} label={location.name}>{location.name}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>
			{#if teamState.memberError}
				<p role="alert" class="text-sm text-destructive">{teamState.memberError}</p>
				<Button variant="outline" size="sm" onclick={() => teamState.loadMembers()}>{text.refresh}</Button>
			{:else}
				<Card.Root class="gap-0 py-0" data-testid="team-employee-page">
					<Card.Content class="divide-y p-0">
						{#each teamState.members as member (member.memberID)}
							<button type="button" class="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-muted/50" onclick={() => { selectedMember = member; sheetOpen = true; }}>
								<PersonAvatar name={member.name} email={member.email} class="size-9" />
								<span class="min-w-0 flex-1"><span class="block truncate font-medium">{member.name}</span><span class="block truncate text-xs text-muted-foreground">{member.email}</span></span>
								<span class="hidden min-w-0 items-center gap-1 text-xs text-muted-foreground sm:flex"><MapPinIcon class="size-3" />{member.location ?? '—'}</span>
								<Badge variant={member.status === 'working' ? 'default' : 'secondary'}>{statusName(member.status)}</Badge>
							</button>
						{:else}
							<p class="p-5 text-sm text-muted-foreground">{teamState.isLoadingMembers ? text.teamEmployees + '…' : text.noMembers}</p>
						{/each}
					</Card.Content>
				</Card.Root>
				<div class="flex items-center justify-between gap-3 text-sm text-muted-foreground">
					<span>{teamState.memberTotal ? teamState.memberOffset + 1 : 0}–{Math.min(teamState.memberOffset + 24, teamState.memberTotal)} / {teamState.memberTotal}</span>
					<div class="flex gap-2">
						<Button variant="outline" size="sm" disabled={teamState.memberOffset === 0 || teamState.isLoadingMembers} onclick={() => teamState.loadMembers(Math.max(0, teamState.memberOffset - 24))}>{text.teamPagePrevious}</Button>
						<Button variant="outline" size="sm" disabled={teamState.memberOffset + 24 >= teamState.memberTotal || teamState.isLoadingMembers} onclick={() => teamState.loadMembers(teamState.memberOffset + 24)}>{text.teamPageNext}</Button>
					</div>
				</div>
			{/if}
		{/if}
	</div>

	<Sheet.Root bind:open={sheetOpen}>
		<Sheet.Content side="right" class="w-full sm:max-w-md">
			{#if selectedMember}
				<Sheet.Header>
					<Sheet.Title class="flex items-center gap-3"><PersonAvatar name={selectedMember.name} email={selectedMember.email} class="size-10" />{selectedMember.name}</Sheet.Title>
					<Sheet.Description>{selectedMember.email}</Sheet.Description>
				</Sheet.Header>
				<div class="space-y-4 px-4">
					<Badge>{statusName(selectedMember.status)}</Badge>
					<p class="flex items-center gap-2 text-sm"><Clock3Icon class="size-4" />{selectedMember.latestAt ? timeOf(selectedMember.latestAt) : '—'}</p>
					<p class="flex items-center gap-2 text-sm"><MapPinIcon class="size-4" />{selectedMember.location ?? '—'}</p>
					<Button variant="outline" onclick={() => { sheetOpen = false; if (selectedMember) onOpenMonthly(selectedMember); }}>{text.teamMonthlyOpen}</Button>
				</div>
			{/if}
		</Sheet.Content>
	</Sheet.Root>
</Tooltip.Provider>
