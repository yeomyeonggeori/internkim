<script lang="ts">
	import { displayPersonName } from '$lib/person-name.svelte';
 import ListPaginationFooter from "$lib/components/list-pagination-footer.svelte";
	import { supabaseAttendanceTodaySummary } from '$lib/attendance/supabase-attendance';
	import type { AttendanceSummary } from '../attendance-context.svelte';
	import { buildTeamRowsForDates } from './team-status-table-model';
	import { companyDateOf, companyTimeOf } from '$lib/company-time';
	import AttendancePersonRow from './attendance-person-row.svelte';
	import TeamStatusDayProgress from './team-status-day-progress.svelte';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import TeamPersonMonth from './team-person-month.svelte';
	import { invokeTool, ToolRefused } from '$lib/public-api-call';
	import { appNavigation } from '$lib/components/app-navigation.svelte';
	import { getAttendanceViewState } from '../attendance-view-state.svelte';
	import { onMount, untrack } from 'svelte';
	import { leaveOpenedTeamThroughHistory, rememberOpenedTeam, teamKeyInHistory } from './attendance-team-history';
	import AttendanceLoadingSkeleton from '../attendance-loading-skeleton.svelte';
	import AttendanceCompanyMetrics from './attendance-company-metrics.svelte';
	import AttendanceTeamCard from './attendance-team-card.svelte';
	import SearchIcon from '@lucide/svelte/icons/search';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
	import Clock3Icon from '@lucide/svelte/icons/clock-3';
	import LocationLabel from '../shared/location-label.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
 import OwnClockAction from './own-clock-action.svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Empty from '$lib/components/ui/empty';
	import * as InputGroup from '$lib/components/ui/input-group';
	import * as Select from '$lib/components/ui/select';
	import * as Sheet from '$lib/components/ui/sheet';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import type { AttendanceTeamPage } from '$lib/attendance/team-page';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { companyScopeKey, getAttendanceTeamState, type MemberStatusFilter } from './attendance-team-state.svelte';
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import MemberCell from '$lib/components/member/member-cell.svelte';
	import { memberRolesOf, type MemberProfile } from '$lib/components/member/member-role';
	import { memberText } from '$lib/components/member/member-text';
	import { openDirectMessageWith } from '$lib/messenger/open-direct-message';
	import { supabaseOrganizationDirectory } from '$lib/organization/supabase-directory';
	import { responsibleMemberIDOf } from '$lib/organization/responsible-member';
	import type { UsersResponse } from '$lib/organization/types';
	import AttendanceMoment from '../shared/attendance-moment.svelte';
	import { memberStatusOf } from './team-card-status';

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
	const words = createPageText(memberText);
	let searchDraft = $state('');
	let locationDraft = $state('');
	let statusDraft = $state<MemberStatusFilter | ''>('');
	let directory = $state<UsersResponse | undefined>(undefined);
	const isCompanyScope = $derived(teamState.selectedTeamKey === companyScopeKey);
	const recordByMember = $derived(new Map((directory?.records ?? []).map((record) => [record.memberID, record])));
	const groupNameByID = $derived(new Map((directory?.availableGroups ?? []).map((group) => [group.id, group.name])));
	const responsibleByGroup = $derived(new Map((directory?.availableGroups ?? []).map((group) => [group.id, responsibleMemberIDOf((directory?.records ?? []).filter((record) => record.groupID === group.id))])));
	const statusOptions = $derived([
		{ value: 'working' as const, label: text.working },
		{ value: 'done' as const, label: text.finished },
		{ value: 'away' as const, label: text.onLeave },
		{ value: 'not_started' as const, label: text.teamNotStarted }
	]);
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

	$effect(() => {
		if (!teamState.selectedTeamKey || directory) return;
		supabaseOrganizationDirectory()
			.then((found) => (directory = found))
			.catch((failure: unknown) => console.warn('the organization directory did not load for the employee page', failure));
	});

	function memberProfileOf(member: AttendanceTeamPage['members'][number]): MemberProfile {
		const record = recordByMember.get(member.memberID);
		const groupID = record?.groupID ?? '';
		return {
			memberID: member.memberID,
			name: member.name,
			email: member.email,
			image: record?.image,
			jobTitle: record?.jobTitle,
			team: groupNameByID.get(groupID),
			phoneNumber: record?.phoneNumber,
			roles: memberRolesOf(member.memberID, directory?.representativeID, responsibleByGroup.get(groupID))
		};
	}

	async function messageMember(member: MemberProfile): Promise<void> {
		const opening = await openDirectMessageWith(member, page.url.pathname);
		if (opening === 'no-messenger-account') toast.error(words.noMessengerAccount.replace('{name}', member.name));
	}

	$effect(() => {
		if (!teamState.selectedTeamKey) return;
		const search = searchDraft.trim();
		const location = locationDraft;
		const status = statusDraft;
		if (search === teamState.searchText && location === teamState.locationFilter && status === teamState.statusFilter) return;
		const timer = setTimeout(() => teamState.filter(search, location, status), 250);
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


	function selectTeam(key: string, status: MemberStatusFilter | '' = ''): void {
		searchDraft = '';
		locationDraft = '';
		statusDraft = status;
		teamState.selectTeam(key, status);
		rememberOpenedTeam(key);
	}

	function backToTeams(): void {
		if (leaveOpenedTeamThroughHistory()) return;
		dismissTeam();
	}

	function dismissTeam(): void {
		teamState.clearSelection();
		searchDraft = '';
		locationDraft = '';
		statusDraft = '';
	}

	$effect(() => {
		if (teamKeyInHistory() === undefined && untrack(() => teamState.selectedTeamKey) !== '') dismissTeam();
	});
</script>

<Tooltip.Provider>
	<div class="mx-auto flex w-full max-w-7xl flex-col gap-6 max-sm:gap-4" data-testid="attendance-team-dashboard" aria-busy={teamState.isLoadingTeams || teamState.isLoadingMembers}>

        {#if myAttendanceToday.loadFailure}<p role="alert" class="text-sm text-destructive">{myAttendanceToday.loadFailure}</p>{/if}
        <Card.Root class="gap-0 py-0" data-testid="attendance-own-strip" aria-busy={myAttendanceToday.isLoading}>
            <AttendancePersonRow name={ownName} email={ownEmail} status={ownStatusKind} statusLabel={ownStatus} day={ownDay} location={ownLocation === '—' ? null : ownLocation} onclick={openOwnRecord}>
                {#snippet action()}<OwnClockAction />{/snippet}
            </AttendancePersonRow>
        </Card.Root>
        {#if !teamState.selectedTeamKey && teamState.isLoadingTeams && !companyTotals && !teamState.error}
            <AttendanceLoadingSkeleton kind="metrics" />
        {:else if !teamState.selectedTeamKey && companyTotals}
			<AttendanceCompanyMetrics summary={companyTotals} onSelect={(status) => selectTeam(companyScopeKey, status)} />
        {/if}
		<div class="flex flex-wrap items-end justify-between gap-3">
			<div>
				{#if teamState.selectedTeamKey}
					<Button variant="ghost" size="sm" class="-ml-2 mb-1" onclick={backToTeams}>
						<ArrowLeftIcon />{text.teamBack}
					</Button>
				{/if}
				<h2 class="text-xl font-semibold tracking-tight max-sm:text-lg">{selectedTeam ? teamName(selectedTeam) : isCompanyScope ? text.teamWholeCompany : text.teamTodayTitle}</h2>
			</div>
			<div class="flex items-center gap-3"><span class="text-xs text-muted-foreground">{teamState.serverTime ? timeOf(teamState.serverTime) : '—'} 기준</span></div>
		</div>

		{#if !teamState.selectedTeamKey}
			{#if teamState.error}
				<p role="alert" class="text-sm text-destructive">{text.teamLoadFailed} {teamState.error}</p>
				<Button variant="outline" size="sm" onclick={() => teamState.loadTeams()}>{text.refresh}</Button>
			{/if}
			{#if (teamState.isLoadingTeams || !teamState.hasLoadedTeams) && !teamState.teams.length && !teamState.error}
				<AttendanceLoadingSkeleton kind="teams" />
			{:else if !teamState.teams.length && !teamState.error}
				<Empty.Root class="border">
					<Empty.Header><Empty.Title>{text.noTeams}</Empty.Title></Empty.Header>
				</Empty.Root>
			{:else if teamState.teams.length || !teamState.error}
				<div class="grid gap-4 max-sm:gap-3 min-[761px]:grid-cols-2 min-[1101px]:grid-cols-3" data-testid="team-card-page">
					{#each teamState.teams as team (team.teamKey)}
						<AttendanceTeamCard {team} name={teamName(team)} isCallerTeam={team.isCallerTeam} onStatus={(status) => selectTeam(team.teamKey, memberStatusOf(status))}>
							{#snippet face(person)}<PersonAvatar name={person.name} email={person.email} memberID={person.memberID} class="size-6" />{/snippet}
							{#snippet footer()}
								<Button variant="ghost" class="h-11 w-full justify-between rounded-none px-4 text-sm font-medium" onclick={() => selectTeam(team.teamKey)}>
									{text.teamAllMembers.replace('{count}', String(team.memberCount))}<ArrowRightIcon class="size-4" />
								</Button>
							{/snippet}
						</AttendanceTeamCard>
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
				<Select.Root type="single" value={statusDraft || '__all'} onValueChange={(value) => (statusDraft = statusOptions.find((option) => option.value === value)?.value ?? '')}>
					<Select.Trigger class="w-full sm:w-40" aria-label={text.allStatuses}>{statusOptions.find((option) => option.value === statusDraft)?.label ?? text.allStatuses}</Select.Trigger>
					<Select.Content><Select.Group>
						<Select.Item value="__all" label={text.allStatuses}>{text.allStatuses}</Select.Item>
						{#each statusOptions as option (option.value)}
							<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
						{/each}
					</Select.Group></Select.Content>
				</Select.Root>
			</div>
			{#if teamState.memberError}
				<p role="alert" class="text-sm text-destructive">{teamState.memberError}</p>
				<Button variant="outline" size="sm" onclick={() => teamState.loadMembers()}>{text.refresh}</Button>
			{/if}
			{#if (teamState.isLoadingMembers || !teamState.hasLoadedMembers) && !teamState.members.length && !teamState.memberError}
				<AttendanceLoadingSkeleton kind="members" />
			{:else if teamState.members.length || !teamState.memberError}
				{#if todayError}<p role="alert" class="text-sm text-destructive">{todayError}</p>{/if}
				<Card.Root class="gap-0 py-0" data-testid="team-employee-page">
					<Card.Content class="flex flex-col gap-0.5 p-2">
						{#each teamState.members as member (member.memberID)}
							{@const profile = memberProfileOf(member)}
							{@const day = todayDays.get(member.memberID)}
							<MemberCell
								member={profile}
								showsTeam={isCompanyScope}
								onSelect={() => { selectedMember = member; sheetOpen = true; }}
								onMessage={() => void messageMember(profile)}
								testID={`team-employee-${member.memberID}`}
							>
								{#snippet detail()}
									<AttendanceMoment status={member.status} latestAt={member.latestAt} location={member.location} timeZone={teamState.timeZone} />
									{#if day || todayLoading}<span class="mt-1 block w-full" aria-busy={todayLoading}>{#if day}<TeamStatusDayProgress {day} />{:else}<Skeleton class="h-1.5 w-full rounded-full" data-testid="employee-today-progress-loading" aria-hidden="true" />{/if}</span>{/if}
								{/snippet}
							</MemberCell>
						{:else}
							<Empty.Root>
								<Empty.Header><Empty.Title>{teamState.searchText.trim() || teamState.locationFilter || teamState.statusFilter ? text.noMatchingMembers : text.noMembers}</Empty.Title></Empty.Header>
								{#if teamState.searchText.trim() || teamState.locationFilter || teamState.statusFilter}
									<Empty.Content><Button variant="outline" size="sm" onclick={() => { searchDraft = ''; locationDraft = ''; statusDraft = ''; teamState.filter('', '', ''); }}>{text.clearFilters}</Button></Empty.Content>
								{/if}
							</Empty.Root>
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
