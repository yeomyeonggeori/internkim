const dayInMilliseconds = 86_400_000;

function dayCountBetween(firstDate: string, lastDate: string): number {
	const first = Date.parse(`${firstDate}T00:00:00Z`);
	const last = Date.parse(`${lastDate}T00:00:00Z`);
	if (Number.isNaN(first) || Number.isNaN(last) || last < first) return 0;
	return Math.round((last - first) / dayInMilliseconds) + 1;
}

export function leaveDaysInYear(
	totalDays: number,
	localStartDate: string,
	localEndDate: string,
	targetYear: number
): number {
	const lastDate = localEndDate < localStartDate ? localStartDate : localEndDate;
	const span = dayCountBetween(localStartDate, lastDate);
	if (span === 0) return 0;

	const startYear = Number(localStartDate.slice(0, 4));
	const endYear = Number(lastDate.slice(0, 4));
	if (startYear === endYear) return startYear === targetYear ? totalDays : 0;
	if (targetYear < startYear || targetYear > endYear) return 0;

	const firstOfTarget = `${targetYear}-01-01`;
	const lastOfTarget = `${targetYear}-12-31`;
	const inYear = dayCountBetween(
		localStartDate > firstOfTarget ? localStartDate : firstOfTarget,
		lastDate < lastOfTarget ? lastDate : lastOfTarget
	);
	return (totalDays * inYear) / span;
}
