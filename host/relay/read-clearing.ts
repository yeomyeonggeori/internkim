import { messageTagOf } from './arrived';

export const readMarkCapability = 'person.read_state.mark';

export type ReadClearing = {
	memberID: string;
	tag: string;
};

export function readClearingOf(memberID: string, body: Record<string, unknown>): ReadClearing | null {
	const conversationID = typeof body.conversationID === 'string' ? body.conversationID.trim() : '';
	if (!memberID || !conversationID) return null;
	return { memberID, tag: messageTagOf(conversationID) };
}

export type ReadClearingTellerDependencies = {
	askTheProject: (request: ReadClearing) => Promise<{ reached?: number }>;
	report: (line: string) => void;
};

export function readClearingTeller(dependencies: ReadClearingTellerDependencies): (memberID: string, body: Record<string, unknown>) => Promise<number> {
	return async (memberID, body) => {
		const clearing = readClearingOf(memberID, body);
		if (!clearing) return 0;
		try {
			const answered = await dependencies.askTheProject(clearing);
			return answered.reached ?? 0;
		} catch (failure) {
			dependencies.report(`notifications of a read conversation not cleared: ${failure instanceof Error ? failure.message : String(failure)}`);
			return 0;
		}
	};
}
