export type MemberScoreScale = {
	lowestValue: number;
	highestValue: number;
};

const minimumBarWidth = 18;
const emptyBarWidth = 4;

export function memberScoreScale(values: number[]): MemberScoreScale {
	const scoredValues = values.filter((value) => value > 0);
	if (scoredValues.length === 0) return { lowestValue: 0, highestValue: 0 };
	return { lowestValue: Math.min(...scoredValues), highestValue: Math.max(...scoredValues) };
}

export function memberScoreWidth(value: number, scale: MemberScoreScale): number {
	if (value <= 0) return emptyBarWidth;
	const span = scale.highestValue - scale.lowestValue;
	if (span <= 0) return 100;
	const ratio = (value - scale.lowestValue) / span;
	return Math.round(minimumBarWidth + Math.max(0, Math.min(1, ratio)) * (100 - minimumBarWidth));
}
