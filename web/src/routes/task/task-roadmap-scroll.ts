import { taskRoadmapMonthKey, taskRoadmapMonthOf, type TaskRoadmapMonth, type TaskRoadmapMonthSegment } from './task-roadmap-model';

const visibleMonthProbe = 1 / 3;

export function taskRoadmapTodayScrollDay(segments: TaskRoadmapMonthSegment[], todayIndex: number, visibleDays: number): number {
	const todayDay = Math.max(todayIndex, 0);
	const todaySegment = segments.find((segment) => todayDay >= segment.firstDay && todayDay < segment.firstDay + segment.dayCount);
	const probeOffset = Math.floor(visibleDays * visibleMonthProbe);
	return Math.max(todaySegment?.firstDay ?? 0, todayDay - probeOffset);
}

export function taskRoadmapMonthScrollDay(segments: TaskRoadmapMonthSegment[], month: TaskRoadmapMonth): number | undefined {
	const key = taskRoadmapMonthKey(month);
	return segments.find((segment) => taskRoadmapMonthKey(segment.month) === key)?.firstDay;
}

export function taskRoadmapVisibleMonth(days: string[], scrolledDays: number, visibleDays: number): TaskRoadmapMonth | undefined {
	const probeDay = Math.floor(scrolledDays + visibleDays * visibleMonthProbe);
	const dayISO = days[Math.min(Math.max(probeDay, 0), days.length - 1)];
	return dayISO ? taskRoadmapMonthOf(dayISO) : undefined;
}
