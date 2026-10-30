import type { CompanyEvent } from '../company-event';

export const workingEventKind = 'agent.working';
export const saidEventKind = 'agent.said';
export const liveWorkLifetimeMilliseconds = 120_000;
export const toolStepStatuses = ['pending', 'in_progress', 'completed', 'failed'] as const;

export type ToolStepStatus = (typeof toolStepStatuses)[number];

export type ToolStep = { title: string; status: ToolStepStatus };

export type WorkReport = {
	threadID: string;
	deliveryID?: string;
	promptID?: string;
	steps?: ToolStep[];
	messageID?: string;
	isStillWorking?: boolean;
};

export type LiveWork = {
	key: string;
	threadID: string;
	steps: ToolStep[];
	heardAt: number;
	becomingMessageID?: string;
	isStillWorking?: boolean;
};

export type ShownWork = { key: string; steps: ToolStep[] };

const promptKeyPrefix = 'prompt:';

export function liveWorkAfterEvent(
	works: LiveWork[],
	event: CompanyEvent,
	conversationID: string | undefined,
	now: number
): LiveWork[] {
	if (!conversationID || event.conversationID !== conversationID) return works;
	const report = workReportOf(event.work);
	if (!report) return works;
	if (event.kind === workingEventKind) return worksAfterWorking(works, report, now);
	if (event.kind === saidEventKind) return worksAfterSaid(works, report);
	return works;
}

export function liveWorkStillHeard(works: LiveWork[], now: number): LiveWork[] {
	const heard = works.filter((work) => now - work.heardAt < liveWorkLifetimeMilliseconds);
	return heard.length === works.length ? works : heard;
}

export function liveWorkShownIn(
	works: LiveWork[],
	threadID: string | undefined,
	loadedMessageIDs: ReadonlySet<string>
): ShownWork[] {
	if (threadID === undefined) return [];
	return works
		.filter((work) => work.threadID === threadID)
		.flatMap((work) => {
			if (work.becomingMessageID === undefined || !loadedMessageIDs.has(work.becomingMessageID)) {
				return [{ key: work.key, steps: work.steps }];
			}
			return work.isStillWorking ? [{ key: work.key, steps: [] }] : [];
		});
}

function worksAfterWorking(works: LiveWork[], report: WorkReport, now: number): LiveWork[] {
	const key = keyOf(report);
	if (key === undefined) return works;
	const heard: LiveWork = { key, threadID: report.threadID, steps: report.steps ?? [], heardAt: now };
	if (works.some((work) => work.key === key)) return works.map((work) => (work.key === key ? heard : work));
	const provisional = works.find((work) => work.key.startsWith(promptKeyPrefix) && work.threadID === report.threadID);
	if (report.deliveryID && provisional) return works.map((work) => (work === provisional ? heard : work));
	return [...works, heard];
}

function worksAfterSaid(works: LiveWork[], report: WorkReport): LiveWork[] {
	const key = keyOf(report);
	if (key === undefined) return works;
	if (report.messageID) {
		return works.map((work) =>
			work.key === key ? { ...work, becomingMessageID: report.messageID, isStillWorking: report.isStillWorking === true } : work
		);
	}
	if (report.isStillWorking) return works;
	return works.filter((work) => work.key !== key);
}

function keyOf(report: WorkReport): string | undefined {
	if (report.deliveryID) return report.deliveryID;
	if (report.promptID) return `${promptKeyPrefix}${report.promptID}`;
	return undefined;
}

function workReportOf(offered: unknown): WorkReport | null {
	if (typeof offered !== 'object' || offered === null) return null;
	const { threadID, deliveryID, promptID, steps, messageID, isStillWorking } = offered as Record<string, unknown>;
	if (typeof threadID !== 'string' || threadID === '') return null;
	return {
		threadID,
		...(typeof deliveryID === 'string' && deliveryID ? { deliveryID } : {}),
		...(typeof promptID === 'string' && promptID ? { promptID } : {}),
		...(Array.isArray(steps) ? { steps: steps.flatMap(toolStepOf) } : {}),
		...(typeof messageID === 'string' && messageID ? { messageID } : {}),
		...(typeof isStillWorking === 'boolean' ? { isStillWorking } : {})
	};
}

function toolStepOf(offered: unknown): ToolStep[] {
	if (typeof offered !== 'object' || offered === null) return [];
	const { title, status } = offered as Record<string, unknown>;
	if (typeof title !== 'string' || !isToolStepStatus(status)) return [];
	return [{ title, status }];
}

function isToolStepStatus(offered: unknown): offered is ToolStepStatus {
	return toolStepStatuses.some((status) => status === offered);
}
