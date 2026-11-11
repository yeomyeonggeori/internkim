export const taskRoadmapColumnsClass =
	'grid grid-cols-[15.5rem_2.25rem_minmax(0,1fr)_2.25rem] lg:grid-cols-[20rem_2.25rem_minmax(0,1fr)_2.25rem] max-sm:grid-cols-[2.25rem_minmax(0,1fr)_2.25rem]';

const redDayShade = 'color-mix(in srgb, var(--muted) 45%, transparent)';

export type TaskRoadmapDaySpan = {
	firstDay: number;
	lastDay: number;
};

function percentOf(dayCount: number, totalDays: number): string {
	return `${(dayCount / Math.max(1, totalDays)) * 100}%`;
}

export function taskRoadmapSpanStyle(span: TaskRoadmapDaySpan, totalDays: number): string {
	const left = percentOf(span.firstDay, totalDays);
	const width = percentOf(span.lastDay - span.firstDay + 1, totalDays);
	return `left: calc(${left} + 2px); width: calc(${width} - 4px)`;
}

export function taskRoadmapTodayLineLeft(todayIndex: number, totalDays: number): string {
	return percentOf(todayIndex + 0.5, totalDays);
}

export function taskRoadmapRedDayBackground(redDays: boolean[]): string {
	const stops = redDays.map((red, index) => `${red ? redDayShade : 'transparent'} ${percentOf(index, redDays.length)} ${percentOf(index + 1, redDays.length)}`);
	return `linear-gradient(to right, ${stops.join(', ')})`;
}
