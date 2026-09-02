export type ContractedTool = { resultContract?: unknown };

export function statesAResultContract(tool: ContractedTool): boolean {
	return tool.resultContract !== undefined;
}
