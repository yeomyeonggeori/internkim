<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import PersonCard from './person-card.svelte';
	import type { AttendancePresence } from '../attendance-context.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { computePeopleToday, uniquePeople } from '../shared/attendance-aggregation';
	import { todayDateInTimeZone } from '../shared/attendance-date';
	import { attendanceText } from '../text';

	type PresenceFilter = AttendancePresence | 'all';
	type PresencePerson = {
		email: string;
		displayName: string;
		presence: AttendancePresence;
	};

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);
	let presenceFilter = $state<PresenceFilter>('all');

	const presenceOptions = $derived<{ value: PresenceFilter; label: string }[]>([
		{ value: 'all', label: text.all },
		{ value: 'online', label: text.online },
		{ value: 'away', label: text.away },
		{ value: 'dnd', label: text.dnd },
		{ value: 'offline', label: text.offline },
	]);

	const displayDate = $derived(
		attendance.selectedDate || resolveDefaultDate(attendance.summary?.month, attendance.summary?.events ?? [])
	);
	const today = $derived(todayDateInTimeZone(attendance.summary?.timeZone));
	const isToday = $derived(displayDate === today);
	const isViewingCurrentMonth = $derived(
		!!attendance.summary && attendance.summary.month === today.slice(0, 7)
	);
	const people = $derived(
		attendance.summary ? computePeopleToday(displayDate, attendance.summary.events, undefined, today) : []
	);
	const counts = $derived(summarize(people));
	const presencePeople = $derived(
		buildPresencePeople(attendance.summary?.events ?? [], attendance.summary?.presences ?? {})
	);
	const filteredPresencePeople = $derived(
		presenceFilter === 'all'
			? presencePeople
			: presencePeople.filter((person) => person.presence === presenceFilter)
	);
	const presenceCounts = $derived(summarizePresences(presencePeople));

	function resolveDefaultDate(
		month: string | undefined,
		events: { localDate: string; kind: string; canceledAt?: string }[]
	): string {
		if (!month) return today;
		if (today.startsWith(month)) return today;
		const dates = events
			.filter((event) => event.kind === 'clock_in' && !event.canceledAt && event.localDate.startsWith(month))
			.map((event) => event.localDate)
			.sort();
		if (dates.length) return dates[0];
		return `${month}-01`;
	}

	function summarize(list: ReturnType<typeof computePeopleToday>) {
		let working = 0;
		let finished = 0;
		let absent = 0;
		let weekend = 0;
		let upcoming = 0;
		for (const p of list) {
			if (p.status === 'working') working += 1;
			else if (p.status === 'finished') finished += 1;
			else if (p.status === 'weekend') weekend += 1;
			else if (p.status === 'upcoming') upcoming += 1;
			else absent += 1;
		}
		return { working, finished, absent, weekend, upcoming };
	}

	function buildPresencePeople(
		events: Parameters<typeof uniquePeople>[0],
		presences: Record<string, AttendancePresence>
	): PresencePerson[] {
		return uniquePeople(events)
			.map((person) => {
				const presence = presences[person.email];
				return presence ? { email: person.email, displayName: person.displayName, presence } : null;
			})
			.filter((person): person is PresencePerson => person !== null);
	}

	function summarizePresences(list: PresencePerson[]): Record<AttendancePresence, number> {
		const counts: Record<AttendancePresence, number> = {
			online: 0,
			away: 0,
			dnd: 0,
			offline: 0,
		};
		for (const person of list) {
			counts[person.presence] += 1;
		}
		return counts;
	}

	function presenceOptionCount(value: PresenceFilter): number {
		return value === 'all' ? presencePeople.length : presenceCounts[value];
	}

	function presenceDotClass(presence: AttendancePresence): string {
		switch (presence) {
			case 'online':
				return 'bg-emerald-500';
			case 'away':
				return 'bg-amber-500';
			case 'dnd':
				return 'bg-red-500';
			case 'offline':
				return 'bg-muted-foreground/40';
		}
	}

	function presenceButtonClass(value: PresenceFilter): string {
		const base = 'inline-flex h-8 items-center gap-1.5 rounded-full border px-3 text-xs font-medium transition';
		if (value === presenceFilter) {
			return `${base} border-foreground bg-foreground text-background`;
		}
		return `${base} border-border bg-background text-muted-foreground hover:border-foreground/40 hover:text-foreground`;
	}

	function selectPerson(email: string) {
		attendance.selectedEmail = email;
		attendance.setTab('personal');
	}

	function backToToday() {
		attendance.selectedDate = '';
	}

	function formatHeader(date: string, todayFlag: boolean, currentMonth: boolean): string {
		const d = new Date(`${date}T00:00:00Z`);
		const weekday = weekdayLabel(d.getUTCDay());
		const label = `${d.getUTCMonth() + 1}/${d.getUTCDate()} (${weekday})`;
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
	{#if presencePeople.length > 0}
		<div class="border-t px-6 py-3">
			<div class="flex flex-wrap items-center gap-2">
				<span class="text-xs font-medium text-muted-foreground">{text.currentPresence}</span>
				{#each presenceOptions as option (option.value)}
					<button
						type="button"
						class={presenceButtonClass(option.value)}
						aria-pressed={presenceFilter === option.value}
						onclick={() => (presenceFilter = option.value)}
					>
						{#if option.value !== 'all'}
							<span class={`size-2 rounded-full ${presenceDotClass(option.value)}`}></span>
						{/if}
						{option.label} {presenceOptionCount(option.value)}
					</button>
				{/each}
			</div>
			<div class="mt-2 flex flex-wrap gap-1.5">
				{#each filteredPresencePeople as person (person.email)}
					<span class="inline-flex items-center gap-1.5 rounded-full bg-muted px-2.5 py-1 text-xs text-muted-foreground">
						<span class={`size-2 rounded-full ${presenceDotClass(person.presence)}`}></span>
						{person.displayName}
					</span>
				{/each}
			</div>
		</div>
	{/if}
	<Card.Content class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
		{#each people as person (person.email)}
			<PersonCard {person} onSelect={selectPerson} />
		{/each}
		{#if people.length === 0}
			<p class="col-span-full text-sm text-muted-foreground">{text.noMembers}</p>
		{/if}
	</Card.Content>
</Card.Root>
