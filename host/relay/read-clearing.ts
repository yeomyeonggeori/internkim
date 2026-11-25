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

export type ReadClearingTeller = {
	read: (memberID: string, body: Record<string, unknown>) => void;
	arrived: (conversationID: string) => void;
};

export function readClearingTeller(dependencies: ReadClearingTellerDependencies): ReadClearingTeller {
	const waiting = new Map<string, { tag: string; cancel: () => void }>();
	const clear = async (clearing: ReadClearing): Promise<void> => {
		try {
			await dependencies.askTheProject(clearing);
		} catch (failure) {
			dependencies.report(`notifications of a read conversation not cleared: ${failure instanceof Error ? failure.message : String(failure)}`);
		}
	};
	return {
		read: (memberID, body) => {
			const clearing = readClearingOf(memberID, body);
			if (!clearing) return;
			const key = `${clearing.memberID} ${clearing.tag}`;
			waiting.get(key)?.cancel();
			const cancel = dependencies.schedule(() => {
				waiting.delete(key);
				void clear(clearing);
			}, dependencies.quietMilliseconds);
			waiting.set(key, { tag: clearing.tag, cancel });
		},
		arrived: (conversationID) => {
			const tag = messageTagOf(conversationID);
			for (const [key, pending] of waiting) {
				if (pending.tag !== tag) continue;
				pending.cancel();
				waiting.delete(key);
			}
		}
	};
}
