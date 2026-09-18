export type ContractedTool = { resultContract?: unknown };

export const toolInvokeOutcomes = ['succeeded', 'failed', 'denied'] as const;

export function statesAResultContract(tool: ContractedTool): boolean {
	return tool.resultContract !== undefined;
}
