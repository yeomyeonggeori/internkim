export type SupabaseWorkStatusTimeRange = {
	from: string;
	until: string;
};

export function supabaseWorkStatusTimeRange(
	days: readonly string[],
	timeZone: string
): SupabaseWorkStatusTimeRange {
	const firstDay = days[0];
	const lastDay = days.at(-1);
	if (!firstDay || !lastDay) throw new Error('work status time range requires at least one day');
	return {
		from: companyTimeInstant(firstDay, '00:00', timeZone),
		until: companyTimeInstant(shiftedDay(lastDay, 1), '00:00', timeZone)
	};
}

export function shiftedDay(day: string, days: number): string {
	const moved = new Date(`${day}T00:00:00Z`);
	moved.setUTCDate(moved.getUTCDate() + days);
	return moved.toISOString().slice(0, 10);
}

export function companyTimeInstant(day: string, time: string, timeZone: string): string {
	if (!/^\d{2}:\d{2}$/.test(time)) throw new Error(`invalid company time: ${time}`);
	const [hour, minute] = time.split(':').map(Number);
	if (hour > 23 || minute > 59) throw new Error(`invalid company time: ${time}`);
	const expectedEpoch = new Date(`${day}T${time}:00Z`).getTime();
	if (Number.isNaN(expectedEpoch)) throw new Error(`invalid work status date: ${day}`);

	let resolvedEpoch = expectedEpoch;
	for (let attempt = 0; attempt < 3; attempt += 1) {
		const local = localDateTime(new Date(resolvedEpoch), timeZone);
		const representedEpoch = Date.UTC(
			local.year,
			local.month - 1,
			local.day,
			local.hour,
			local.minute,
			local.second
		);
		const correction = expectedEpoch - representedEpoch;
		if (correction === 0) return new Date(resolvedEpoch).toISOString();
		resolvedEpoch += correction;
	}

	const result = new Date(resolvedEpoch);
	const local = localDateTime(result, timeZone);
	if (
		local.year !== Number(day.slice(0, 4)) ||
		local.month !== Number(day.slice(5, 7)) ||
		local.day !== Number(day.slice(8, 10)) ||
		local.hour !== hour ||
		local.minute !== minute ||
		local.second !== 0
	) {
		throw new Error(`work status date does not exist in company timezone: ${day} ${timeZone}`);
	}
	return result.toISOString();
}

type LocalDateTime = {
	year: number;
	month: number;
	day: number;
	hour: number;
	minute: number;
	second: number;
};

function localDateTime(instant: Date, timeZone: string): LocalDateTime {
	const parts = new Intl.DateTimeFormat('en-CA', {
		timeZone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit',
		hour: '2-digit',
		minute: '2-digit',
		second: '2-digit',
		hourCycle: 'h23'
	}).formatToParts(instant);
	const valueOf = (type: Intl.DateTimeFormatPartTypes): number =>
		Number(parts.find((part) => part.type === type)?.value ?? Number.NaN);
	return {
		year: valueOf('year'),
		month: valueOf('month'),
		day: valueOf('day'),
		hour: valueOf('hour'),
		minute: valueOf('minute'),
		second: valueOf('second')
	};
}
