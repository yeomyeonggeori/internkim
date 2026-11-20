export const failureCauses = ['transient', 'administrator'] as const;

export type FailureCause = (typeof failureCauses)[number];

export function failureCauseOf(value: unknown): FailureCause | undefined {
	return failureCauses.find((cause) => cause === value);
}
