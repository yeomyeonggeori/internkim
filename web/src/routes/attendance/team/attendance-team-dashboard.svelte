<script lang="ts">
 import ListPaginationFooter from "$lib/components/list-pagination-footer.svelte";
	import { supabaseAttendanceTodaySummary } from '$lib/attendance/supabase-attendance';
	import type { AttendanceSummary } from '../attendance-context.svelte';
	import { buildTeamRowsForDates } from './team-status-table-model';
	import { companyDateOf, companyTimeOf } from '$lib/company-time';
	import AttendancePersonRow from './attendance-person-row.svelte';
	import TeamPersonMonth from './team-person-month.svelte';
	import { invokeTool, ToolRefused } from '$lib/public-api-call';
	import { appNavigation } from '$lib/components/app-navigation.svelte';
	import { getAttendanceViewState } from '../attendance-view-state.svelte';
	import { onMount } from 'svelte';
	import AttendanceLoadingSkeleton from '../attendance-loading-skeleton.svelte';
	import { Spinner } from '$lib/components/ui/spinner';
	import SearchIcon from '@lucide/svelte/icons/search';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import PalmtreeIcon from '@lucide/svelte/icons/palmtree';
	import BedIcon from '@lucide/svelte/icons/bed';
	import Clock3Icon from '@lucide/svelte/icons/clock-3';
	import FlameIcon from '@lucide/svelte/icons/flame';
	import LocationLabel from '../shared/location-label.svelte';
	import ColorMarker from '$lib/components/color-marker.svelte';
	import PersonAvatarStack from '$lib/components/person-avatar-stack.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
 import OwnClockAction from './own-clock-action.svelte';
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

	const text = createPageText(attendanceText);
	const teamState = getAttendanceTeamState();
	const attendanceView = getAttendanceViewState();
	let companyLabel = $state('');
	const ownEmail = $derived(myAttendanceToday.summary?.currentUserEmail ?? '');
	const ownName = $derived(appNavigation.displayUserName);
	const ownStatusKind = $derived(myAttendanceToday.activeLeave ? 'away' : myAttendanceToday.status === 'working' ? 'working' : myAttendanceToday.status === 'finished' ? 'done' : 'not_started');
	const ownStatus = $derived(statusName(ownStatusKind));
	const ownLocation = $derived(myAttendanceToday.day.activeSegment?.locationName ?? '—');
	const companyTotals = $derived(teamState.companySummary);
	const companyMetrics = $derived(companyTotals ? [
		{ label: text.working, icon: FlameIcon, count: companyTotals.working },
		{ label: text.finished, icon: LogOutIcon, count: companyTotals.done },
		{ label: text.onLeave, icon: PalmtreeIcon, count: companyTotals.away },
		{ label: text.teamNotStarted, icon: BedIcon, count: companyTotals.notStarted }
	] : []);
	let searchDraft = $state('');
	let locationDraft = $state('');
	let selectedMember = $state<AttendanceTeamPage['members'][number] | null>(null);
	let sheetOpen = $state(false);
	const selectedTeam = $derived(teamState.teams.find((team) => team.teamKey === teamState.selectedTeamKey));
	const knownLocations = $derived(myAttendanceToday.summary?.locations ?? []);

    let todaySummary = $state<AttendanceSummary | null>(null);
    let progressNow = $state(new Date());
    onMount(() => {const timer = setInterval(() => progressNow = new Date(), 60000); return () => clearInterval(timer);});
    const progressDate = $derived(companyDateOf(progressNow, myAttendanceToday.summary?.timeZone ?? 'UTC'));
    const progressTime = $derived(companyTimeOf(progressNow, myAttendanceToday.summary?.timeZone ?? 'UTC'));
    const todayDays = $derived(new Map((todaySummary ? buildTeamRowsForDates([progressDate], todaySummary, text, progressDate, todaySummary, progressTime, progressNow) : []).map(row => [row.memberID, row.days[0]])));
    const ownDay = $derived(myAttendanceToday.summary ? buildTeamRowsForDates([progressDate], myAttendanceToday.summary, text, progressDate, myAttendanceToday.summary, progressTime, progressNow).find(row => row.email === ownEmail)?.days[0] : undefined);
    let todayError = $state('');
    let todayLoading = $state(false);
    let todayScope = '';
    $effect(() => {
        const members = teamState.members;
        const requester = myAttendanceToday.summary;
        teamState.revision;
        let active = true;
        const scope = JSON.stringify([requester?.currentMemberID, requester?.currentUserEmail, requester?.isAdmin, progressDate, members.map(member => member.memberID)]);
        if (scope !== todayScope) { todaySummary = null; todayScope = scope; }
        todayError = '';
        todayLoading = Boolean(requester && members.length);
        if (requester && members.length) void supabaseAttendanceTodaySummary(members.map(member => ({memberID:member.memberID,email:member.email,displayName:member.name})), requester).then(summary => {if(active) todaySummary = summary;}).catch(error => {if(active) { if(error instanceof ToolRefused && [401, 403].includes(error.status)) todaySummary = null; todayError = error instanceof Error ? error.message : String(error); }}).finally(() => {if(active) todayLoading = false;});
        return () => {active = false;};
    });

	onMount(() => { teamState.start(); void invokeTool<{name: string}>('company_settings_get', {includeProfileImage: false}).then((company) => companyLabel = company.name).catch(() => {}); });

	function teamName(team: AttendanceTeamPage['teams'][number]): string {
		return team.teamKey === 'unassigned' ? teamState.companyName || companyLabel || team.name : team.name;
	}

	async function openActor(actor: RecentActor, teamKey: string): Promise<void> {
		try {
			const answer = await invokeTool<AttendanceTeamPage>('attendance_team_page_get', {pageKind: 'members', selectedTeamKey: teamKey, searchText: actor.email, memberLimit: 48});
			const member = answer.members.find((member) => member.memberID === actor.memberID);
			if (member) { selectedMember = member; sheetOpen = true; }
		} catch (error) { teamState.memberError = error instanceof Error ? error.message : String(error); }
	}

	$effect(() => {
		if (!teamState.selectedTeamKey) return;
		const search = searchDraft.trim();
		const location = locationDraft;
		if (search === teamState.searchText && location === teamState.locationFilter) return;
		const timer = setTimeout(() => teamState.filter(search, location), 250);
		return () => clearTimeout(timer);
	});

    function openOwnRecord(): void {
        const own = myAttendanceToday.summary;
        if (!own?.currentMemberID) return;
        selectedMember = {memberID: own.currentMemberID, name: ownName, email: ownEmail, teamKey: '', status: ownStatusKind, latestAt: myAttendanceToday.day.clockIn?.occurredAt ?? null, location: ownLocation === '—' ? null : ownLocation};
        sheetOpen = true;
    }

	function timeOf(instant: string): string {
		return new Intl.DateTimeFormat(text.dateLocale, {
			hour: '2-digit', minute: '2-digit', timeZone: teamState.timeZone || 'UTC'
		}).format(new Date(instant));
	}

	function statusName(status: string): string {
		if (status === 'working') return text.working;
		if (status === 'done') return text.finished;
		if (status === 'away') return text.onLeave;
		if (status === 'needs_checkout') return text.working;
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

{#snippet actorStack(actors: RecentActor[], label: string, teamKey: string)}
	<div class="flex min-w-0 items-center justify-between gap-2">
		<span class="text-xs text-muted-foreground">{label}</span>
		{#if actors.length}
            <PersonAvatarStack people={actors} max={4} label={label} avatarClass="size-8 ring-2 ring-card">
                {#snippet renderPerson(person, index)}
                    {@const actor = actors[index]}
                    <Tooltip.Root>
                        <Tooltip.Trigger aria-label={actor.name + ' ' + timeOf(actor.occurredAt)} onclick={() => openActor(actor, teamKey)} class="relative rounded-full ring-2 ring-card">
                            <PersonAvatar name={person.name} email={person.email} class="size-8" />
                        </Tooltip.Trigger>
                        <Tooltip.Content>{actor.name} · {timeOf(actor.occurredAt)}</Tooltip.Content>
                    </Tooltip.Root>
                {/snippet}
            </PersonAvatarStack>
		{:else}
			<span class="text-xs text-muted-foreground">—</span>
		{/if}
	</div>
{/snippet}

<Tooltip.Provider>
	<div class="mx-auto flex w-full max-w-7xl flex-col gap-6" data-testid="attendance-team-dashboard" aria-busy={teamState.isLoadingTeams || teamState.isLoadingMembers}>

        {#if myAttendanceToday.loadFailure}<p role="alert" class="text-sm text-destructive">{myAttendanceToday.loadFailure}</p>{/if}
        <Card.Root class="gap-0 py-0" data-testid="attendance-own-strip" aria-busy={myAttendanceToday.isLoading}>
            <AttendancePersonRow name={ownName} email={ownEmail} status={ownStatusKind} statusLabel={ownStatus} day={ownDay} location={ownLocation === '—' ? null : ownLocation} onclick={openOwnRecord}>
                {#snippet action()}<OwnClockAction />{/snippet}
            </AttendancePersonRow>
        </Card.Root>
        {#if !teamState.selectedTeamKey && teamState.isLoadingTeams && !companyTotals && !teamState.error}
            <AttendanceLoadingSkeleton kind="metrics" />
        {:else if !teamState.selectedTeamKey && companyTotals}
            <div class="grid grid-cols-2 gap-3 lg:grid-cols-4" data-testid="attendance-company-summary">
                {#each companyMetrics as metric (metric.label)}
                    <Card.Root class="gap-2 py-4">
                        <Card.Header class="px-4"><Card.Description class="flex items-center gap-1.5"><metric.icon class="size-3.5" />{metric.label}</Card.Description></Card.Header>
                        <Card.Content class="flex items-end justify-between px-4"><span class="text-3xl font-semibold tracking-tight tabular-nums">{metric.count}<span class="ml-1 text-sm font-normal text-muted-foreground">/ {companyTotals.memberCount}</span></span><span class="text-sm tabular-nums text-muted-foreground">{Math.round(proportion(metric.count, companyTotals.memberCount))}%</span></Card.Content>
                    </Card.Root>
                {/each}
            </div>
        {/if}
		<div class="flex flex-wrap items-end justify-between gap-3">
			<div>
				{#if teamState.selectedTeamKey}
					<Button variant="ghost" size="sm" class="-ml-2 mb-1" onclick={backToTeams}>
						<ArrowLeftIcon />{text.teamBack}
					</Button>
				{/if}
				<h2 class="text-xl font-semibold tracking-tight">{selectedTeam ? teamName(selectedTeam) : text.teamTodayTitle}</h2>
			</div>
			<div class="flex items-center gap-3">{#if (teamState.isLoadingTeams && teamState.teams.length) || (teamState.isLoadingMembers && teamState.members.length)}<Spinner aria-label={text.refresh} />{/if}<span class="text-xs text-muted-foreground">{teamState.serverTime ? timeOf(teamState.serverTime) : '—'} 기준</span></div>
		</div>

		{#if !teamState.selectedTeamKey}
			{#if teamState.error}
				<p role="alert" class="text-sm text-destructive">{text.teamLoadFailed} {teamState.error}</p>
				<Button variant="outline" size="sm" onclick={() => teamState.loadTeams()}>{text.refresh}</Button>
			{/if}
			{#if teamState.isLoadingTeams && !teamState.teams.length}
				<AttendanceLoadingSkeleton kind="teams" />
			{:else if teamState.teams.length || !teamState.error}
				<div class="grid gap-4 min-[761px]:grid-cols-2 min-[1101px]:grid-cols-3" data-testid="team-card-page">
					{#each teamState.teams as team (team.teamKey)}
						<Card.Root class="gap-0 overflow-hidden rounded-xl py-0" data-testid="attendance-team-card">
							<Card.Header class="gap-3 px-4 pb-3 pt-4">
								<div class="flex items-start justify-between gap-3">
									<div class="min-w-0"><Card.Title class="truncate text-base">{teamName(team)}</Card.Title><p class="mt-1 text-xs text-muted-foreground">{team.memberCount}명</p></div>
									<div class="shrink-0 text-right" aria-label={`${team.working}/${team.memberCount} ${text.working}`}>
										<span class="text-2xl font-semibold leading-none tabular-nums">{Math.round(proportion(team.working, team.memberCount))}%</span>
										<span class="block pt-1 text-xs text-muted-foreground">{text.working} {team.working}/{team.memberCount}</span>
									</div>
								</div>
								<div class="flex h-3 overflow-hidden rounded-full bg-muted" role="img" aria-label={`${text.working} ${team.working}, ${text.finished} ${team.done}, ${text.onLeave} ${team.away}, ${text.teamNotStarted} ${team.notStarted}`}>
									<span class="bg-success" style:width={`${proportion(team.working, team.memberCount)}%`}></span>
									<span class="bg-muted-foreground" style:width={`${proportion(team.done, team.memberCount)}%`}></span>
									<span class="bg-warning" style:width={`${proportion(team.away, team.memberCount)}%`}></span>
								</div>
							</Card.Header>
							<Card.Content class="flex flex-col gap-4 px-4 pb-4">
                                <div class="grid grid-cols-2 gap-x-6 gap-y-2 text-xs text-muted-foreground">
                                    <span class="flex items-center justify-between gap-4"><span class="inline-flex items-center gap-1.5"><ColorMarker color="var(--color-success)" /><span>{text.working}</span></span><strong class="shrink-0 tabular-nums text-foreground">{team.working}</strong></span>
                                    <span class="flex items-center justify-between gap-4"><span class="inline-flex items-center gap-1.5"><ColorMarker color="var(--color-muted-foreground)" /><span>{text.finished}</span></span><strong class="shrink-0 tabular-nums text-foreground">{team.done}</strong></span>
                                    <span class="flex items-center justify-between gap-4"><span class="inline-flex items-center gap-1.5"><ColorMarker color="var(--color-warning)" /><span>{text.onLeave}</span></span><strong class="shrink-0 tabular-nums text-foreground">{team.away}</strong></span>
                                    <span class="flex items-center justify-between gap-4"><span class="inline-flex items-center gap-1.5"><ColorMarker color="var(--color-muted)" /><span>{text.teamNotStarted}</span></span><strong class="shrink-0 tabular-nums text-foreground">{team.notStarted}</strong></span>
                                </div>

								<div class="flex flex-wrap items-center gap-x-2 gap-y-1 border-t border-border/70 pt-3 text-xs text-muted-foreground">
																		<span>{text.teamRecordedLocations}</span>
									{#each team.recordedLocations as location (location.name)}
										<LocationLabel name={location.name} count={location.count} />
									{/each}
									{#if team.unknownLocationCount}<LocationLabel name={text.teamUnknownLocation} count={team.unknownLocationCount} />{/if}
									{#if !team.recordedLocations.length && !team.unknownLocationCount}<span>—</span>{/if}
								</div>
								<div class="flex flex-col gap-3 border-t border-border/70 pt-3">
									{@render actorStack(team.recentClockIns, text.teamRecentIns, team.teamKey)}
									{@render actorStack(team.recentClockOuts, text.teamRecentOuts, team.teamKey)}
								</div>
							</Card.Content>
							<Card.Footer class="border-t border-border/70 bg-card p-0">
								<Button variant="ghost" class="h-11 w-full justify-between rounded-none px-4 text-sm font-medium" onclick={() => selectTeam(team.teamKey)}>
								{text.teamEmployeesView}<ArrowRightIcon class="size-4" />
							</Button>
						</Card.Footer>
						</Card.Root>
					{/each}
				</div>
				<ListPaginationFooter totalItems={teamState.teamTotal} pageIndex={teamState.teamOffset / 6} pageSize={6} pageCount={Math.ceil(teamState.teamTotal / 6)} canPreviousPage={teamState.teamOffset > 0} canNextPage={teamState.teamOffset + 6 < teamState.teamTotal} previousPage={() => void teamState.loadTeams(teamState.teamOffset - 6)} nextPage={() => void teamState.loadTeams(teamState.teamOffset + 6)} onPageChange={(page) => void teamState.loadTeams(page * 6)} disabled={teamState.isLoadingTeams} summary={'{from}–{to} / {total}'} previousLabel={text.teamPagePrevious} nextLabel={text.teamPageNext} showSummary={true} />
			{/if}
		{:else}
			<div class="flex flex-col gap-3 sm:flex-row">
				<InputGroup.Root class="min-w-0 flex-1">
					<InputGroup.Addon align="inline-start"><SearchIcon /></InputGroup.Addon>
					<InputGroup.Input aria-label={text.teamSearch} placeholder={text.teamSearch} bind:value={searchDraft} />
				</InputGroup.Root>
				<Select.Root type="single" value={locationDraft || '__all'} onValueChange={(value) => (locationDraft = value === '__all' ? '' : value)}>
					<Select.Trigger class="w-full sm:w-52">{locationDraft || text.allLocations}</Select.Trigger>
					<Select.Content><Select.Group>
						<Select.Item value="__all" label={text.allLocations}>{text.allLocations}</Select.Item>
						{#each knownLocations as location (location.name)}
							<Select.Item value={location.name} label={location.name}>{location.name}</Select.Item>
						{/each}
					</Select.Group></Select.Content>
				</Select.Root>
			</div>
			{#if teamState.memberError}
				<p role="alert" class="text-sm text-destructive">{teamState.memberError}</p>
				<Button variant="outline" size="sm" onclick={() => teamState.loadMembers()}>{text.refresh}</Button>
			{/if}
			{#if teamState.isLoadingMembers && !teamState.members.length}
				<AttendanceLoadingSkeleton kind="members" />
			{:else if teamState.members.length || !teamState.memberError}
				{#if todayError}<p role="alert" class="text-sm text-destructive">{todayError}</p>{/if}
				<Card.Root class="gap-0 py-0" data-testid="team-employee-page">
					<Card.Content class="divide-y p-0">
						{#each teamState.members as member (member.memberID)}
                            <AttendancePersonRow name={member.name} email={member.email} status={member.status} statusLabel={statusName(member.status)} day={todayDays.get(member.memberID)} progressLoading={todayLoading} location={member.location ?? text.teamUnknownLocation} onclick={() => { selectedMember = member; sheetOpen = true; }} />
						{:else}
							<p class="p-5 text-sm text-muted-foreground">{text.noMembers}</p>
						{/each}
					</Card.Content>
				</Card.Root>
				<ListPaginationFooter totalItems={teamState.memberTotal} pageIndex={teamState.memberOffset / 24} pageSize={24} pageCount={Math.ceil(teamState.memberTotal / 24)} canPreviousPage={teamState.memberOffset > 0} canNextPage={teamState.memberOffset + 24 < teamState.memberTotal} previousPage={() => void teamState.loadMembers(teamState.memberOffset - 24)} nextPage={() => void teamState.loadMembers(teamState.memberOffset + 24)} onPageChange={(page) => void teamState.loadMembers(page * 24)} disabled={teamState.isLoadingMembers} summary={'{from}–{to} / {total}'} previousLabel={text.teamPagePrevious} nextLabel={text.teamPageNext} showSummary={true} />
			{/if}
		{/if}
	</div>

	<Sheet.Root bind:open={sheetOpen}>
		<Sheet.Content side="right" class="flex w-full flex-col overflow-y-auto sm:max-w-2xl">
			{#if sheetOpen && selectedMember}
				<Sheet.Header>
					<Sheet.Title class="flex items-center gap-3"><PersonAvatar name={selectedMember.name} email={selectedMember.email} class="size-10" />{selectedMember.name}</Sheet.Title>
					<Sheet.Description>{selectedMember.email}{selectedMember.teamKey ? ' · ' + (teamState.teams.find(team => team.teamKey === selectedMember?.teamKey) ? teamName(teamState.teams.find(team => team.teamKey === selectedMember?.teamKey)!) : '') : ''}</Sheet.Description>
				</Sheet.Header>
				<div class="space-y-4 px-4">
					{#if selectedMember.status !== 'not_started'}<Badge>{statusName(selectedMember.status)}</Badge>{/if}
					<p class="flex items-center gap-2 text-sm"><Clock3Icon class="size-4" />{selectedMember.latestAt ? timeOf(selectedMember.latestAt) : '—'}</p>
					{#if selectedMember.status === 'working' || selectedMember.status === 'needs_checkout'}<LocationLabel name={selectedMember.location ?? text.teamUnknownLocation} />{/if}
					{#key selectedMember.memberID}<TeamPersonMonth member={selectedMember} />{/key}
				</div>
			{/if}
		</Sheet.Content>
	</Sheet.Root>
</Tooltip.Provider>
