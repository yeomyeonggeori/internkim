import { messageTagOf } from './arrived';

export const readMarkCapability = 'person.read_state.mark';

export const readClearingQuietMilliseconds = 30_000;

export type ReadClearing = {
	memberID: string;
	tag: string;
};

export function readClearingOf(memberID: string, body: Record<string, unknown>): ReadClearing | null {
	const conversationID = typeof body.conversationID === 'string' ? body.conversationID.trim() : '';
	if (!memberID || !conversationID || isThreadRead(body)) return null;
	return { memberID, tag: messageTagOf(conversationID) };
}

function isThreadRead(body: Record<string, unknown>): boolean {
	return typeof body.parentID === 'string' && body.parentID.trim() !== '';
}

export type ReadClearingTellerDependencies = {
	askTheProject: (request: ReadClearing) => Promise<{ reached?: number }>;
	report: (line: string) => void;
	quietMilliseconds: number;
	schedule: (run: () => void, milliseconds: number) => () => void;
};

export function readClearingTeller(dependencies: ReadClearingTellerDependencies): (memberID: string, body: Record<string, unknown>) => void {
	const waiting = new Map<string, () => void>();
	const clear = async (clearing: ReadClearing): Promise<void> => {
		try {
			await dependencies.askTheProject(clearing);
		} catch (failure) {
			dependencies.report(`notifications of a read conversation not cleared: ${failure instanceof Error ? failure.message : String(failure)}`);
		}
	};
	return (memberID, body) => {
		const clearing = readClearingOf(memberID, body);
		if (!clearing) return;
		const key = `${clearing.memberID} ${clearing.tag}`;
		waiting.get(key)?.();
		waiting.set(key, dependencies.schedule(() => {
			waiting.delete(key);
			void clear(clearing);
		}, dependencies.quietMilliseconds));
	};
}
