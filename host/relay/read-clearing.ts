import { messageNotificationTag } from '../../web/src/lib/notifications/message-tag';

export const readMarkCapability = 'person.read_state.mark';

export const readClearingQuietMilliseconds = 30_000;

export type ReadClearing = {
	memberID: string;
	tag: string;
};

export function readClearingOf(memberID: string, body: Record<string, unknown>): ReadClearing | null {
	const conversationID = typeof body.conversationID === 'string' ? body.conversationID.trim() : '';
	if (!memberID || !conversationID || isThreadRead(body)) return null;
	return { memberID, tag: messageNotificationTag(conversationID) };
}

function isThreadRead(body: Record<string, unknown>): boolean {
	return typeof body.parentID === 'string' && body.parentID.trim() !== '';
}

export type ReadClearingTellerDependencies = {
	askTheProject: (request: ReadClearing) => Promise<{ reached?: number }>;
	memberIDsOf: (externalIDs: string[]) => Promise<Map<string, string>>;
	report: (line: string) => void;
	quietMilliseconds: number;
	schedule: (run: () => void, milliseconds: number) => () => void;
};

export type ReadClearingTeller = {
	read: (memberID: string, body: Record<string, unknown>) => void;
	arrived: (conversationID: string, recipientExternalIDs: string[]) => Promise<void>;
};

export function readClearingTeller(dependencies: ReadClearingTellerDependencies): ReadClearingTeller {
	const waiting = new Map<string, { memberID: string; tag: string; cancel: () => void }>();
	const recipientMemberIDsOf = async (externalIDs: string[]): Promise<Set<string> | null> => {
		try {
			return new Set((await dependencies.memberIDsOf(externalIDs)).values());
		} catch (failure) {
			dependencies.report(`recipients of an arrival not resolved, so every clearing waiting on its conversation is dropped: ${failure instanceof Error ? failure.message : String(failure)}`);
			return null;
		}
	};
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
			waiting.set(key, { memberID: clearing.memberID, tag: clearing.tag, cancel });
		},
		arrived: async (conversationID, recipientExternalIDs) => {
			const tag = messageNotificationTag(conversationID);
			const waitingOnTag = [...waiting].filter(([, pending]) => pending.tag === tag);
			if (waitingOnTag.length === 0) return;
			const recipients = await recipientMemberIDsOf(recipientExternalIDs);
			for (const [key, pending] of waitingOnTag) {
				if (waiting.get(key) !== pending) continue;
				if (recipients && !recipients.has(pending.memberID)) continue;
				pending.cancel();
				waiting.delete(key);
			}
		}
	};
}
