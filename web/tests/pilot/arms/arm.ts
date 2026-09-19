export interface ModelCall {
	kind: string;
	promptTokens: number;
	cachedPromptTokens: number;
	completionTokens: number;
	provider: string;
	latencyMs?: number;
	providerReportedCostUSD?: number;
	generationID?: string;
}

export interface HarnessOutcome {
	status: 'completed' | 'failed' | 'timed_out';
	turns: number;
	approvalsAnsweredByRequester: number;
	toolCalls: string[];
	reply: string;
	calls: ModelCall[];
}

export interface ArmRunContext {
	instruction: string;
	evidenceDirectory: string;
	repositoryRoot: string;
	appURL: string;
	personalAccessToken: string;
	modelCredential: string;
	model: string;
	requesterEmail: string;
	fleetConfigurationPath: string;
}

export type ArmName = 'bluecollar' | 'bluecollar-pi-shaped' | 'claude-code';

export const armNames: ArmName[] = ['bluecollar', 'bluecollar-pi-shaped', 'claude-code'];

export function isArmName(candidate: string): candidate is ArmName {
	return (armNames as string[]).includes(candidate);
}
