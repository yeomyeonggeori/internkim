const millisecondsInADay = 24 * 60 * 60 * 1000;

export function dayIn(timezone: string, instant: Date): string {
	return new Intl.DateTimeFormat('en-CA', {
		timeZone: timezone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).format(instant);
}

export function dayOfInstant(timezone: string, instant: string | null): string {
	return instant ? dayIn(timezone, new Date(instant)) : '';
}

function mondayOf(day: string): string {
	const noon = new Date(`${day}T12:00:00Z`);
	const weekday = (noon.getUTCDay() + 6) % 7;
	return new Date(noon.getTime() - weekday * millisecondsInADay).toISOString().slice(0, 10);
}

export function dayShifted(day: string, days: number): string {
	return new Date(new Date(`${day}T12:00:00Z`).getTime() + days * millisecondsInADay)
		.toISOString()
		.slice(0, 10);
}

export type DayWindow = { from: string; to: string };

// A week offset is counted from the week the company is in, so the answer does
// not shift with the caller's own clock.
export function weekWindow(timezone: string, now: Date, weekFrom: number, weekTo: number): DayWindow {
	const [earlier, later] = weekFrom <= weekTo ? [weekFrom, weekTo] : [weekTo, weekFrom];
	const thisMonday = mondayOf(dayIn(timezone, now));
	return {
		from: dayShifted(thisMonday, earlier * 7),
		to: dayShifted(thisMonday, later * 7 + 6)
	};
}

export function windowHoldsDay(window: DayWindow, day: string): boolean {
	return day >= window.from && day <= window.to;
}

function offsetMilliseconds(timezone: string, instant: Date): number {
	const parts = new Intl.DateTimeFormat('en-CA', {
		timeZone: timezone,
		hour12: false,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		second: '2-digit'
	}).formatToParts(instant);
	const held = (name: string) => Number(parts.find((part) => part.type === name)?.value ?? '0');
	const asIfUTC = Date.UTC(
		held('year'),
		held('month') - 1,
		held('day'),
		held('hour') % 24,
		held('minute'),
		held('second')
	);
	// A formatted time carries no milliseconds, so the instant it is compared
	// against must not either, or the offset comes back short by them.
	return asIfUTC - Math.floor(instant.getTime() / 1000) * 1000;
}

// A day names a day where the company is, so its edges are that day's midnights
// there. The offset is read twice because the first reading uses a guess that
// can sit on the wrong side of a daylight-saving change.
export function instantOfDay(timezone: string, day: string, endOfDay = false): string {
	const wall = Date.parse(`${day}T${endOfDay ? '23:59:59.999' : '00:00:00.000'}Z`);
	const guessed = new Date(wall - offsetMilliseconds(timezone, new Date(wall)));
	return new Date(wall - offsetMilliseconds(timezone, guessed)).toISOString();
}

// A caller writes either a moment or a whole day. A moment already says where it
// is; a whole day is read where the company is.
export function instantWritten(timezone: string, written: string, endOfDay = false): string {
	const asked = written.trim();
	if (/^\d{4}-\d{2}-\d{2}$/.test(asked)) return instantOfDay(timezone, asked, endOfDay);
	const moment = new Date(asked);
	if (Number.isNaN(moment.getTime())) throw new Error(`${written} is not a date or a moment`);
	return moment.toISOString();
}
