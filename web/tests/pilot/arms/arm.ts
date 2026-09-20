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

export type HarnessStatus = 'completed' | 'failed' | 'timed_out' | 'waiting_user_input';

export interface DeliveredFile {
	filename: string;
	contentType: string;
	sizeBytes: number;
	devicePath: string;
	isZipContainer: boolean | null;
}

export interface HarnessOutcome {
	status: HarnessStatus;
	reachedTheLoop: boolean;
	turns: number;
	toolCalls: string[];
	reply: string;
	deliveredFiles: DeliveredFile[];
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

export type ArmName = 'bluecollar' | 'bluecollar-pi-shaped' | 'bluecollar-reply-action' | 'claude-code';

export const armNames: ArmName[] = ['bluecollar', 'bluecollar-pi-shaped', 'bluecollar-reply-action', 'claude-code'];

export function isArmName(candidate: string): candidate is ArmName {
	return (armNames as string[]).includes(candidate);
}
