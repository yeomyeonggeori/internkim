// Dev Flow mock의 점수 계산용 기간 업무 생성을 담당합니다.
import { addDays, monthStartISO } from './dev-flow-fixture-date';
import type { FlowTask, FlowWeek } from './flow-types';

type FlowTaskFactory = (week: FlowWeek) => FlowTask[];

export function createDevFlowScoreTasks(week: FlowWeek, createTasks: FlowTaskFactory): FlowTask[] {
	return scoreFixtureWeekStarts(week.startISO).flatMap((startISO, index) => createTasks(scoreFixtureWeek(startISO, `${week.code}-score-${index}`)));
}

function scoreFixtureWeekStarts(weekStartISO: string): string[] {
	const starts = new Set<string>();
	for (let index = 0; index < 5; index += 1) {
		starts.add(addDays(weekStartISO, -7 * index));
		starts.add(addMonthsISO(monthStartISO(weekStartISO), -index));
	}
	return Array.from(starts);
}

function scoreFixtureWeek(startISO: string, code: string): FlowWeek {
	return {
		code,
		startISO,
		endISO: addDays(startISO, 6),
		previous: code,
		next: code,
		isCurrent: false
	};
}

function addMonthsISO(value: string, months: number): string {
	const date = new Date(`${value}T00:00:00.000Z`);
	return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + months, 1)).toISOString().slice(0, 10);
}
