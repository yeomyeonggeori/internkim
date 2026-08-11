export type WorkStandardCapacityStage =
	| 'baseline-buffer'
	| 'working-days'
	| 'calendar-days';

type WorkStandardCapacityInput = {
	actualSeconds: number;
	provisionalSeconds: number;
	targetSeconds: number;
	workingCapacitySeconds: number;
	calendarCapacitySeconds: number;
	hasBaseline: boolean;
};

type WorkStandardCapacity = {
	stage: WorkStandardCapacityStage;
	capacitySeconds: number;
	actualWidthPercent: number;
	targetPositionPercent?: number;
};

const baselineCapacityMultiplier = 1.25;
const secondsPerDay = 24 * 60 * 60;

export function calculateCalendarCapacitySeconds(periodStart: string, periodEnd: string): number {
	const start = Date.parse(`${periodStart}T00:00:00Z`);
	const end = Date.parse(`${periodEnd}T00:00:00Z`);
	if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) {
		return secondsPerDay;
	}
	return (Math.floor((end - start) / (secondsPerDay * 1000)) + 1) * secondsPerDay;
}

export function calculateWorkStandardCapacity(
	input: WorkStandardCapacityInput
): WorkStandardCapacity {
	const actualSeconds = Math.max(0, input.actualSeconds) + Math.max(0, input.provisionalSeconds);
	const targetSeconds = Math.max(0, input.targetSeconds);
	const baselineCapacitySeconds = Math.ceil(targetSeconds * baselineCapacityMultiplier);
	const workingCapacitySeconds = Math.max(0, input.workingCapacitySeconds);
	const calendarCapacitySeconds = Math.max(workingCapacitySeconds, input.calendarCapacitySeconds, 1);

	let stage: WorkStandardCapacityStage;
	let capacitySeconds: number;
	if (input.hasBaseline && baselineCapacitySeconds > 0 && actualSeconds <= baselineCapacitySeconds) {
		stage = 'baseline-buffer';
		capacitySeconds = baselineCapacitySeconds;
	} else if (workingCapacitySeconds > 0 && actualSeconds <= workingCapacitySeconds) {
		stage = 'working-days';
		capacitySeconds = workingCapacitySeconds;
	} else {
		stage = 'calendar-days';
		capacitySeconds = calendarCapacitySeconds;
	}

	return {
		stage,
		capacitySeconds,
		actualWidthPercent: Math.min((actualSeconds / capacitySeconds) * 100, 100),
		targetPositionPercent: input.hasBaseline && targetSeconds > 0
			? Math.min((targetSeconds / capacitySeconds) * 100, 100)
			: undefined
	};
}
