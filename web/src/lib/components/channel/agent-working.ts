export const agentReplyTimeoutMs = 120_000;
export const agentWorkingRefreshIntervalMs = 1500;

export function stillWorkingSince(workingSince: number | null, now: number): number | null {
	if (workingSince === null) return null;
	if (now - workingSince >= agentReplyTimeoutMs) return null;
	return workingSince;
}
