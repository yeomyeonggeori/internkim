import { formatHoursMinutes } from '../shared/attendance-format';

type WorkStatusDurationUnits = {
	hourUnit: string;
	minuteUnit: string;
};

export function formatWorkStatusDuration(
	minutes: number,
	units: WorkStatusDurationUnits
): string {
	return (
		formatHoursMinutes(minutes, units) ||
		`00${units.hourUnit} 00${units.minuteUnit}`
	);
}
