export type WorkedClock = { kind: string; occurred_at: string };

type ReadClock = { kind: string; at: number; date: string; time: string };

const minutesAShiftMayLast = 24 * 60;

export function closedWorkedMinutesOn(day: string, clocks: WorkedClock[], timeZone: string): number {
	const ordered = clocks
		.map((clock) => readClock(clock, timeZone))
		.filter((clock): clock is ReadClock => clock !== undefined)
		.sort((left, right) => left.at - right.at);

	let worked = 0;
	let open: ReadClock | undefined;
	for (const clock of ordered) {
		if (clock.kind === 'clock_in') {
			if (open && !outlivedItsDay(open, clock)) worked += minutesOn(day, open, clock);
			open = clock;
			continue;
		}
		if (clock.kind !== 'clock_out' || !open) continue;
		if (!outlivedItsDay(open, clock)) worked += minutesOn(day, open, clock);
		open = undefined;
	}
	return worked;
}

function minutesOn(day: string, from: ReadClock, to: ReadClock): number {
	if (day < from.date || day > to.date) return 0;
	const start = day === from.date ? from.time : '00:00';
	const end = day === to.date ? to.time : '24:00';
	return Math.max(0, localMinutes(end) - localMinutes(start));
}

function outlivedItsDay(from: ReadClock, to: ReadClock): boolean {
	return Math.round((to.at - from.at) / 60_000) > minutesAShiftMayLast;
}

export function companyDayOf(moment: Date, timeZone: string): string {
	return new Intl.DateTimeFormat('en-CA', {
		timeZone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).format(moment);
}

function readClock(clock: WorkedClock, timeZone: string): ReadClock | undefined {
	const at = new Date(clock.occurred_at).getTime();
	if (Number.isNaN(at)) return undefined;
	const parts = new Intl.DateTimeFormat('en-CA', {
		timeZone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		hourCycle: 'h23'
	}).formatToParts(new Date(at));
	const held = (type: string) => parts.find((part) => part.type === type)?.value ?? '';
	return {
		kind: clock.kind,
		at,
		date: `${held('year')}-${held('month')}-${held('day')}`,
		time: `${held('hour')}:${held('minute')}`
	};
}

function localMinutes(localTime: string): number {
	const [hours = 0, minutes = 0] = localTime.split(':').map(Number);
	return hours * 60 + minutes;
}
