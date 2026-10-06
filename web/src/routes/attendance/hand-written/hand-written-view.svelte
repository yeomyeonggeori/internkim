<script lang="ts">
 import ListPaginationFooter from "$lib/components/list-pagination-footer.svelte";
	import { onMount } from 'svelte';
	import AttendanceListLoading from '../attendance-list-loading.svelte';
	import { Spinner } from '$lib/components/ui/spinner';
	import { invokeTool } from '$lib/public-api-call';
	import FilterCombobox from '$lib/components/filter-combobox.svelte';
	import * as Select from '$lib/components/ui/select';
	import type { AttendanceTeamPage } from '$lib/attendance/team-page';
	import { getAttendanceTeamState } from '../team/attendance-team-state.svelte';
	import CalendarDaysIcon from '@lucide/svelte/icons/calendar-days';
	import * as Popover from '$lib/components/ui/popover';
	import { RangeCalendar } from '$lib/components/ui/range-calendar';
	import { parseDate, type DateValue } from '@internationalized/date';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { attendanceChangeReasonLabel } from '$lib/attendance/change-reason';
	import ColoredOutlineBadge from '$lib/components/colored-outline-badge.svelte';
	import LocationLabel from '../shared/location-label.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import Undo2Icon from '@lucide/svelte/icons/undo-2';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Empty from '$lib/components/ui/empty';
	import * as Table from '$lib/components/ui/table';
	import { MediaQuery } from 'svelte/reactivity';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import type { AttendanceKind } from '../attendance-context.svelte';
	import { attendanceText } from '../text';
	import { getHandWrittenState } from './hand-written-state.svelte';
	import { canUndoHandWrittenRecord, type HandWrittenRecord } from './hand-written-records';

	const text = createPageText(attendanceText);
	const handWritten = getHandWrittenState();
	const isMobile = new MediaQuery('(max-width: 639px)');

 const teamState = getAttendanceTeamState();
 let teamOptions = $state<AttendanceTeamPage['teams']>(teamState.teams);
 let teamOptionOffset = $state(0);
 let teamOptionTotal = $state(teamState.teamTotal);
 let peopleOptions = $state<{value:string;label:string;email:string;keywords:string[]}[]>([]);
 let isSearchingPeople = $state(true);
 let peopleSearchError = $state('');
 let selectedPerson = $state<{value:string;label:string;email:string;keywords:string[]}>();
 const personOptions = $derived(selectedPerson && !peopleOptions.some(p=>p.value===selectedPerson?.value) ? [selectedPerson,...peopleOptions] : peopleOptions);
 let disposed = false;
 let peopleSequence = 0;
 let searchTimer: ReturnType<typeof setTimeout>;
 async function loadPeople(query = '') {
  const sequence = ++peopleSequence;
  isSearchingPeople = true;
  peopleSearchError = '';
  try {const answer = await invokeTool<{people:{personID:string;name:string;email:string}[]}>('person_list',{searchText:query,limit:24});
   if(disposed || sequence !== peopleSequence) return;
   peopleOptions = answer.people.map(person=>({value:person.personID,label:person.name,email:person.email,keywords:[person.name,person.email]}));
  } catch {if(!disposed && sequence === peopleSequence) peopleSearchError = text.handWritten.peopleLoadFailed;}
  finally {if(!disposed && sequence === peopleSequence) isSearchingPeople = false;}
 }
 function searchPeople(query: string) {
  peopleSequence++;
  clearTimeout(searchTimer);
  peopleOptions = [];
  isSearchingPeople = true;
  peopleSearchError = '';
  searchTimer = setTimeout(()=>void loadPeople(query),250);
 }
 async function loadTeamOptions(offset = 0) {try {const page = await invokeTool<AttendanceTeamPage>('attendance_team_page_get',{pageKind:'teams',teamOffset:offset,teamLimit:24});if(disposed) return;teamOptions=page.teams;teamOptionOffset=offset;teamOptionTotal=page.teamTotal;} catch {}}
 onMount(()=>{if(teamOptions.length === 0 || teamOptionTotal > teamOptions.length) void loadTeamOptions();void loadPeople();return ()=>{disposed=true;peopleSequence++;clearTimeout(searchTimer);};});

 let periodOpen = $state(false);
 let pickedRange = $state<{start: DateValue | undefined; end: DateValue | undefined}>({start: undefined, end: undefined});
 $effect(() => {
  const {from,to} = handWritten.dayRange;
  if(from && to) pickedRange = {start: parseDate(from),end: parseDate(to)};
 });
 function periodLabel(): string {
  const {from,to} = handWritten.dayRange;
  if(!from || !to) return text.handWritten.period;
  const formatter = new Intl.DateTimeFormat(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US', {month:'short',day:'numeric',timeZone:'UTC'});
  return `${formatter.format(new Date(from+'T12:00:00Z'))} – ${formatter.format(new Date(to+'T12:00:00Z'))}`;
 }

	function kindLabel(kind: AttendanceKind): string {
		return kind === 'clock_in' ? text.clockIn : text.clockOut;
	}

	function wasMoved(record: HandWrittenRecord): boolean {
		return record.originalDate !== null && record.originalTime !== null;
	}

	function undoTitle(record: HandWrittenRecord): string {
		return wasMoved(record) ? text.handWritten.undoMovedTitle : text.handWritten.undoAddedTitle;
	}

	function undoDescription(record: HandWrittenRecord): string {
		const moved = wasMoved(record);
		const template = moved
			? text.handWritten.undoMovedDescriptionTemplate
			: text.handWritten.undoAddedDescriptionTemplate;
		return template
			.replace('{person}', record.person)
			.replace('{kind}', kindLabel(record.kind))
			.replace('{date}', moved ? (record.originalDate ?? '') : record.date)
			.replace('{time}', moved ? (record.originalTime ?? '') : record.time);
	}

	function undoReason(record: HandWrittenRecord): string {
		return wasMoved(record) ? text.handWritten.undoReasonMoved : text.handWritten.undoReasonAdded;
	}
</script>

{#snippet undoControl(record: HandWrittenRecord)}
	<AlertDialog.Root>
		<Tooltip.Provider><Tooltip.Root><Tooltip.Trigger>
            {#snippet child({ props })}
        <AlertDialog.Trigger
            {...props}
            aria-label={`${text.handWritten.undo} · ${record.person} · ${record.date} ${record.time}`}
			class={buttonVariants({ variant: 'ghost', size: 'icon-sm' })}
			disabled={handWritten.undoingEventID !== '' || !canUndoHandWrittenRecord(record)}
			data-testid="hand-written-undo"
		>
            <Undo2Icon class="size-4" />
		</AlertDialog.Trigger>
            {/snippet}
        </Tooltip.Trigger><Tooltip.Content>{canUndoHandWrittenRecord(record) ? text.handWritten.undo : text.handWritten.unknownPreviousRecord}</Tooltip.Content></Tooltip.Root></Tooltip.Provider>
		<AlertDialog.Content data-testid="hand-written-undo-dialog">
			<AlertDialog.Header>
				<AlertDialog.Title>{undoTitle(record)}</AlertDialog.Title>
				<AlertDialog.Description>
					{undoDescription(record)}
      {#if wasMoved(record) && record.kind === 'clock_in' && record.previousRecorded && record.originalLocation}<span class="mt-2 block"><LocationLabel name={record.originalLocation} /></span>{/if}
				</AlertDialog.Description>
			</AlertDialog.Header>
			<AlertDialog.Footer>
				<AlertDialog.Cancel>{text.cancel}</AlertDialog.Cancel>
				<AlertDialog.Action
					data-testid="hand-written-undo-confirm"
					onclick={() => void handWritten.undo(record, undoReason(record))}
				>
					{text.handWritten.undo}
				</AlertDialog.Action>
			</AlertDialog.Footer>
		</AlertDialog.Content>
	</AlertDialog.Root>
{/snippet}

{#snippet changeValue(record: HandWrittenRecord, before: boolean)}
 {@const date = before ? record.originalDate : record.date}
 {@const time = before ? record.originalTime : record.time}
 <div class="grid justify-items-start gap-1.5">
  {#if before && !wasMoved(record)}<span class="text-xs text-muted-foreground">{record.changedBySource === 'observed' ? '—' : text.handWritten.unknownPreviousRecord}</span>{:else}
   <div class="grid gap-0.5 tabular-nums" title={`${date} ${time}`}><span class="text-xs text-muted-foreground">{date}</span><span class="font-medium">{time}</span></div>
   {#if record.kind === 'clock_out'}<ColoredOutlineBadge>{text.clockOut}</ColoredOutlineBadge>
   {:else if before}
    {#if record.previousRecorded && record.originalLocation}<LocationLabel name={record.originalLocation} />{:else if !record.previousRecorded}<span class="max-w-36 whitespace-normal text-xs text-muted-foreground">{text.handWritten.unknownPreviousLocation}</span>{/if}
   {:else if record.location}<LocationLabel name={record.location} />{/if}
  {/if}
 </div>
{/snippet}

{#snippet distinctActor(record: HandWrittenRecord)}
 {#if record.changedByID && record.personID && record.changedByID !== record.personID}
  <div class="mt-1 ml-9 flex items-center gap-1.5 text-xs text-muted-foreground"><PersonAvatar name={record.changedByName ?? ''} email={record.changedByEmail ?? ''} memberID={record.changedByID} class="size-4" /><span aria-label={`${text.handWritten.actor}: ${record.changedByName || '—'}`}>{record.changedByName || '—'}</span></div>
 {/if}
{/snippet}

<section class="mx-auto min-h-0 w-full max-w-7xl space-y-5" data-testid="hand-written-view">
	<header class="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{text.handWritten.title}</h1>
			<p class="mt-1 text-sm text-muted-foreground">{text.handWritten.description}</p>
			<p class="mt-1 text-xs text-muted-foreground" data-testid="hand-written-day-range">
				{handWritten.appliedDayRange.from} — {handWritten.appliedDayRange.to}
			</p>
		</div>
		<Button
			variant={handWritten.isLoading ? 'secondary' : 'ghost'}
			size="icon"
			aria-label={text.handWritten.refresh}
			aria-busy={handWritten.isLoading}
			onclick={() => void handWritten.load()}
			disabled={handWritten.isLoading}
			data-testid="hand-written-refresh"
		>
			{#if handWritten.isLoading}<Spinner aria-label={text.loading} />{:else}<RefreshCwIcon />{/if}
		</Button>
	</header>

	{#if handWritten.errorMessage}
		<p class="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
			{handWritten.errorMessage}
		</p>
	{/if}


 <form class="flex flex-wrap items-center gap-2" aria-label={text.handWritten.filter} onsubmit={(event) => {event.preventDefault(); void handWritten.filter();}}>

  <div class="order-1 w-full sm:order-2 sm:w-52"><FilterCombobox value={handWritten.selectedChangedByID} options={personOptions} label={text.handWritten.actor} searchPlaceholder={text.handWritten.actor} class="h-9 w-full" contentClass="w-72 p-0" remoteSearch isSearching={isSearchingPeople} searchError={peopleSearchError} searchStatusLabel={text.loading} onSearchChange={searchPeople} onSelect={(id)=>{handWritten.selectedChangedByID=id;selectedPerson=personOptions.find(person=>person.value===id);}}>
   {#snippet optionContent(person)}<div class="flex min-w-0 items-center gap-2"><PersonAvatar name={person.label} email={person.email} memberID={person.value} class="size-5" /><span class="truncate">{person.label}</span></div>{/snippet}
   {#snippet selectedContent(person)}<PersonAvatar name={person.label} email={person.email} memberID={person.value} class="size-4" /><span class="truncate">{person.label}</span>{/snippet}
  </FilterCombobox></div>
  <div class="order-2 w-28 sm:order-1 sm:w-40"><Select.Root type="single" value={handWritten.selectedTeamKey || 'all'} onValueChange={(value)=>handWritten.selectedTeamKey=value === 'all' ? '' : value}><Select.Trigger class="h-9 w-full" aria-label={text.handWritten.team}>{teamOptions.find(team=>team.teamKey===handWritten.selectedTeamKey)?.name || (handWritten.selectedTeamKey ? teamState.teams.find(team=>team.teamKey===handWritten.selectedTeamKey)?.name : text.handWritten.team) || text.handWritten.team}</Select.Trigger><Select.Content><Select.Group><Select.Item value="all" label={text.handWritten.allTeams}>{text.handWritten.allTeams}</Select.Item>{#each teamOptions as team (team.teamKey)}<Select.Item value={team.teamKey} label={team.name}>{team.name}</Select.Item>{/each}</Select.Group>{#if teamOptionTotal > 24}<div class="border-t p-2"><ListPaginationFooter totalItems={teamOptionTotal} pageIndex={teamOptionOffset / 24} pageSize={24} pageCount={Math.ceil(teamOptionTotal / 24)} canPreviousPage={teamOptionOffset > 0} canNextPage={teamOptionOffset + 24 < teamOptionTotal} previousPage={() => void loadTeamOptions(teamOptionOffset - 24)} nextPage={() => void loadTeamOptions(teamOptionOffset + 24)} onPageChange={(page) => void loadTeamOptions(page * 24)} disabled={false} summary={'{from}–{to} / {total}'} previousLabel={text.handWritten.previousPage} nextLabel={text.handWritten.nextPage} showSummary={false} /></div>{/if}</Select.Content></Select.Root></div>
  <div class="order-3 min-w-0 flex-1 sm:flex-none"><Popover.Root bind:open={periodOpen}><Popover.Trigger class={buttonVariants({variant:'outline',class:'h-9 w-full justify-start gap-2 sm:w-52'})} aria-label={`${text.handWritten.period}: ${handWritten.dayRange.from} – ${handWritten.dayRange.to}`}><CalendarDaysIcon class="size-4 shrink-0" /><span class="truncate">{periodLabel()}</span></Popover.Trigger><Popover.Content class="w-auto p-0" align="start"><RangeCalendar value={pickedRange} locale={currentLocale.value === 'ko' ? 'ko-KR' : 'en-US'} onValueChange={(range) => {pickedRange = range; if(range.start && range.end){handWritten.dayRange = {from:range.start.toString(),to:range.end.toString()}; periodOpen = false;}}} /></Popover.Content></Popover.Root></div>
  <Button type="submit" class="order-4 h-9" variant="outline" disabled={handWritten.isLoading}>{text.handWritten.filter}</Button>
 </form>


	<Card.Root>
		<Card.Content class="overflow-x-auto pt-6" aria-busy={handWritten.isLoading}>
			{#if handWritten.errorMessage && !handWritten.records.length}
				<p role="status" class="sr-only">{handWritten.errorMessage}</p>
			{:else if (handWritten.isLoading || !handWritten.appliedDayRange.from) && !handWritten.records.length}
				<AttendanceListLoading kind="changes" />
			{:else if handWritten.records.length === 0}
				<Empty.Root data-testid="hand-written-empty">
					<Empty.Header><Empty.Title>{handWritten.hasAppliedFilters ? text.handWritten.filteredEmpty : text.handWritten.empty}</Empty.Title></Empty.Header>
				</Empty.Root>
			{:else if isMobile.current}
				<ul class="divide-y">
					{#each handWritten.records as record (record.eventID)}
						<li class="grid min-w-0 gap-3 py-4" data-testid="hand-written-row" data-event-id={record.eventID}>
							<div class="flex items-start justify-between gap-3"><div class="min-w-0"><div class="flex items-center gap-2"><PersonAvatar name={record.person} email={record.personEmail ?? ''} memberID={record.personID} class="size-7" /><p class="break-words font-medium">{record.person}</p></div><p class="ml-9 text-xs text-muted-foreground">{record.teamName || "—"}</p></div>{@render undoControl(record)}</div>
							{@render distinctActor(record)}
       <dl class="grid grid-cols-2 gap-3 text-sm">
								<div><dt class="text-xs text-muted-foreground">{text.handWritten.before}</dt><dd data-testid="hand-written-before">{@render changeValue(record, true)}</dd></div><div><dt class="text-xs text-muted-foreground">{text.handWritten.now}</dt><dd data-testid="hand-written-now">{@render changeValue(record, false)}</dd></div>
								<div class="col-span-2"><dt class="text-xs text-muted-foreground">{text.handWritten.reason}</dt><dd class="break-words">{attendanceChangeReasonLabel(record.reason, currentLocale.value)}</dd></div>
							</dl>
						</li>
					{/each}
				</ul>
			{:else}
				<Table.Root class="w-full table-fixed">
					<Table.Header>
						<Table.Row>
							<Table.Head class="w-[28%] px-4">{text.handWritten.person}</Table.Head>

							<Table.Head class="w-[22%] px-4">{text.handWritten.before}</Table.Head>
							<Table.Head class="w-[22%] px-4">{text.handWritten.now}</Table.Head>
							<Table.Head class="px-4">{text.handWritten.reason}</Table.Head>
							<Table.Head class="w-12 text-right"><span class="sr-only">{text.handWritten.undo}</span></Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each handWritten.records as record (record.eventID)}
							<Table.Row data-testid="hand-written-row" data-event-id={record.eventID}>
								<Table.Cell class="px-4 py-4 align-top"><div class="flex items-center gap-2"><PersonAvatar name={record.person} email={record.personEmail ?? ''} memberID={record.personID} class="size-7" /><div><p class="font-medium">{record.person}</p><p class="text-xs text-muted-foreground">{record.teamName || '—'}</p></div></div>{@render distinctActor(record)}</Table.Cell>

								<Table.Cell class="px-4 py-4 align-top" data-testid="hand-written-before">{@render changeValue(record, true)}</Table.Cell><Table.Cell class="px-4 py-4 align-top" data-testid="hand-written-now">{@render changeValue(record, false)}</Table.Cell>
								<Table.Cell class="px-4 py-4 align-top whitespace-normal text-sm text-muted-foreground">
									{attendanceChangeReasonLabel(record.reason, currentLocale.value)}
								</Table.Cell>
								<Table.Cell class="text-right">
									{@render undoControl(record)}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			{/if}
		</Card.Content>
	</Card.Root>
 <ListPaginationFooter totalItems={handWritten.totalCount} pageIndex={handWritten.pageOffset / 24} pageSize={24} pageCount={Math.ceil(handWritten.totalCount / 24)} canPreviousPage={handWritten.pageOffset > 0} canNextPage={handWritten.pageOffset + 24 < handWritten.totalCount} previousPage={() => void handWritten.page(handWritten.pageOffset - 24)} nextPage={() => void handWritten.page(handWritten.pageOffset + 24)} onPageChange={(page) => void handWritten.page(page * 24)} disabled={handWritten.isLoading} summary={'{from}–{to} / {total}'} previousLabel={text.handWritten.previousPage} nextLabel={text.handWritten.nextPage} showSummary={true} />
</section>
