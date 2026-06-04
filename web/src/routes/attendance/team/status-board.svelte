<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import PersonCard from './person-card.svelte';
	import PresenceFilter from './presence-filter.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { computePeopleToday } from '../shared/attendance-aggregation';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import { attendanceText } from '../text';
	import {
		buildPresencePeople,
		filterPresencePeople,
		type PresenceFilter as PresenceFilterValue,
		resolveDefaultDate,
		summarizePeople,
		summarizePresences
	} from './status-board-model';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);
	let presenceFilter = $state<PresenceFilterValue>('all');

	type PresenceFilterOption = {
		value: PresenceFilterValue;
		label: string;
	};

	const presenceOptions = $derived<PresenceFilterOption[]>([
		{ value: 'all', label: text.all },
		{ value: 'online', label: text.online },
		{ value: 'away', label: text.away },
		{ value: 'dnd', label: text.dnd },
		{ value: 'offline', label: text.offline },
	]);

	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));
	const displayDate = $derived(
		attendance.selectedDate || resolveDefaultDate(attendance.summary?.month, attendance.summary?.events ?? [], today)
	);
	const isToday = $derived(displayDate === today);
	const isViewingCurrentMonth = $derived(
		!!attendance.summary && attendance.summary.month === today.slice(0, 7)
	);
	const people = $derived(
		attendance.summary ? computePeopleToday(displayDate, attendance.summary.events, undefined, today) : []
	);
	const counts = $derived(summarizePeople(people));
	const presencePeople = $derived(
		buildPresencePeople(attendance.summary?.events ?? [], attendance.summary?.presences ?? {})
	);
	const filteredPresencePeople = $derived(filterPresencePeople(presencePeople, presenceFilter));
	const presenceCounts = $derived(summarizePresences(presencePeople));

	function selectPerson(email: string) {
		attendance.selectedEmail = email;
		attendance.setTab('personal');
	}

	function backToToday() {
		attendance.selectedDate = '';
	}

	function selectPresenceFilter(value: PresenceFilterValue) {
		presenceFilter = value;
	}

	function formatHeader(date: string, todayFlag: boolean, currentMonth: boolean): string {
		const dateValue = new Date(`${date}T00:00:00Z`);
		const weekday = weekdayLabel(dateValue.getUTCDay());
		const label = `${dateValue.getUTCMonth() + 1}/${dateValue.getUTCDate()} (${weekday})`;
		if (todayFlag) return text.todayDateTemplate.replace('{date}', label);
		return currentMonth
			? text.selectedDayTemplate.replace('{date}', label)
			: text.firstClockInDayTemplate.replace('{date}', label);
	}

	function weekdayLabel(day: number): string {
		const labels = [
			text.weekdaySunday,
			text.weekdayMonday,
			text.weekdayTuesday,
			text.weekdayWednesday,
			text.weekdayThursday,
			text.weekdayFriday,
			text.weekdaySaturday,
		];
		return labels[day] ?? '';
	}
</script>

<Card.Root>
	<Card.Header class="flex flex-row items-center justify-between">
		<div>
			<Card.Title class="text-base">{formatHeader(displayDate, isToday, isViewingCurrentMonth)}</Card.Title>
			<p class="text-xs text-muted-foreground">
				{text.working} {counts.working} · {text.finished} {counts.finished}
				{#if counts.absent > 0}
					· {text.absent} {counts.absent}
				{/if}
				{#if counts.upcoming > 0}
					· {text.upcoming} {counts.upcoming}
				{/if}
			</p>
		</div>
		{#if attendance.selectedDate}
			<Button variant="ghost" size="sm" onclick={backToToday}>{text.defaultView}</Button>
		{/if}
	</Card.Header>
	<PresenceFilter
		options={presenceOptions}
		selected={presenceFilter}
		people={presencePeople}
		filteredPeople={filteredPresencePeople}
		{presenceCounts}
		label={text.currentPresence}
		onSelect={selectPresenceFilter}
	/>
	<Card.Content class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
		{#each people as person (person.email)}
			<PersonCard {person} onSelect={selectPerson} />
		{/each}
		{#if people.length === 0}
			<p class="col-span-full text-sm text-muted-foreground">{text.noMembers}</p>
		{/if}
	</Card.Content>
</Card.Root>
