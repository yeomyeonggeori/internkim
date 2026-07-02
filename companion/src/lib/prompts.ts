export type PromptRequest = {
	requestID: string;
	kind: 'confirm' | 'input' | 'approval';
	message: string;
	default?: boolean;
	toolName?: string;
	capabilityScope?: string;
	resourceScope?: { kind?: string; value?: string };
	timeoutSeconds?: number;
};

export type PromptResult = {
	status: 'idle' | 'pending' | 'completed' | 'failed';
	message?: string;
};

export function normalizePromptRequest(value: unknown): PromptRequest {
	if (!isRecord(value)) {
		throw new Error('Prompt request is invalid');
	}
	const rawRequest = value;
	if (typeof rawRequest.requestId !== 'string') {
		throw new Error('Prompt request is missing request ID');
	}
	if (rawRequest.kind !== 'confirm' && rawRequest.kind !== 'input' && rawRequest.kind !== 'approval') {
		throw new Error('Prompt request type is unsupported');
	}
	if (typeof rawRequest.message !== 'string') {
		throw new Error('Prompt request is missing message');
	}
	return {
		requestID: rawRequest.requestId,
		kind: rawRequest.kind,
		message: rawRequest.message,
		default: typeof rawRequest.default === 'boolean' ? rawRequest.default : undefined,
		toolName: typeof rawRequest.toolName === 'string' ? rawRequest.toolName : undefined,
		capabilityScope: typeof rawRequest.capabilityScope === 'string' ? rawRequest.capabilityScope : undefined,
		resourceScope: normalizeResourceScope(rawRequest.resourceScope),
		timeoutSeconds: typeof rawRequest.timeoutSeconds === 'number' ? rawRequest.timeoutSeconds : undefined
	};
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null;
}

export function confirmResponse(isConfirmed: boolean): { confirmed: boolean } {
	return { confirmed: isConfirmed };
}

export function inputResponse(text: string): { text: string } {
	return { text };
}

export function approvalResponse(isAllowed: boolean, userReason: string, rememberSession = false): { allowed: boolean; userReason?: string; suggestedConstraint?: string; rememberSession?: boolean } {
	const trimmedReason = userReason.trim();
	return {
		allowed: isAllowed,
		rememberSession: isAllowed && rememberSession ? true : undefined,
		userReason: isAllowed ? undefined : trimmedReason || undefined,
		suggestedConstraint: isAllowed ? undefined : trimmedReason || undefined
	};
}

function normalizeResourceScope(value: unknown): { kind?: string; value?: string } | undefined {
	if (!isRecord(value)) return undefined;
	return {
		kind: typeof value.kind === 'string' ? value.kind : undefined,
		value: typeof value.value === 'string' ? value.value : undefined
	};
}
