<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import PersonCard from './person-card.svelte';
	import type { AttendancePresence } from '../attendance-context.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { computePeopleToday, uniquePeople } from '../shared/attendance-aggregation';
	import { todayDateInTimeZone } from '../shared/attendance-date';

	type PresenceFilter = AttendancePresence | 'all';
	type PresencePerson = {
		email: string;
		displayName: string;
		presence: AttendancePresence;
	};

	const PRESENCE_OPTIONS: { value: PresenceFilter; label: string }[] = [
		{ value: 'all', label: '전체' },
		{ value: 'online', label: '온라인' },
		{ value: 'away', label: '자리비움' },
		{ value: 'dnd', label: '방해 금지' },
		{ value: 'offline', label: '오프라인' },
	];

	const attendance = getAttendanceState();
	let presenceFilter = $state<PresenceFilter>('all');

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
		attendance.tab = 'personal';
	}

	function backToToday() {
		attendance.selectedDate = '';
	}

	function formatHeader(date: string, todayFlag: boolean, currentMonth: boolean): string {
		const d = new Date(`${date}T00:00:00Z`);
		const weekday = ['일', '월', '화', '수', '목', '금', '토'][d.getUTCDay()];
		const label = `${d.getUTCMonth() + 1}/${d.getUTCDate()} (${weekday})`;
		if (todayFlag) return `오늘 ${label}`;
		return currentMonth ? `선택된 날 ${label}` : `${label} (첫 출근일)`;
	}
</script>

<Card.Root>
	<Card.Header class="flex flex-row items-center justify-between">
		<div>
			<Card.Title class="text-base">{formatHeader(displayDate, isToday, isViewingCurrentMonth)}</Card.Title>
			<p class="text-xs text-muted-foreground">
				근무 {counts.working} · 퇴근 {counts.finished}
				{#if counts.absent > 0}
					· 미출근 {counts.absent}
				{/if}
				{#if counts.upcoming > 0}
					· 예정 {counts.upcoming}
				{/if}
			</p>
		</div>
		{#if attendance.selectedDate}
			<Button variant="ghost" size="sm" onclick={backToToday}>기본으로</Button>
		{/if}
	</Card.Header>
	{#if presencePeople.length > 0}
		<div class="border-t px-6 py-3">
			<div class="flex flex-wrap items-center gap-2">
				<span class="text-xs font-medium text-muted-foreground">현재 상태</span>
				{#each PRESENCE_OPTIONS as option (option.value)}
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
			<p class="col-span-full text-sm text-muted-foreground">표시할 구성원이 없습니다.</p>
		{/if}
	</Card.Content>
</Card.Root>
