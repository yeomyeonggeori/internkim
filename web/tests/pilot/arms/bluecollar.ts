import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
import type { ArmRunContext, DeliveredFile, HarnessOutcome, HarnessStatus, ModelCall } from './arm';

const guestScriptPath = '/mnt/shared/workspace/lab/scripts/pilot-bluecollar-task.py';
const resultMarker = 'PILOT-RESULT ';
const guestPassword = 'admin';
const runTimeoutMs = 900_000;

interface LLMCallRecord {
	kind: string;
	promptTokens?: number;
	cachedPromptTokens?: number;
	completionTokens?: number;
	upstreamProvider?: string;
	latencyMs?: number;
	costUSD?: number;
}

interface GuestDeliveredFile {
	filename?: string;
	contentType?: string;
	sizeBytes?: number;
	devicePath?: string;
	isZipContainer?: boolean | null;
}

interface GuestResult {
	status: string;
	taskRunID: string;
	requestedTools: string[];
	llmCalls: LLMCallRecord[];
	deliveredFiles: GuestDeliveredFile[];
	result: string;
	failureReason: string;
	turns: number;
	detail: unknown;
}

function modelCallOf(record: LLMCallRecord): ModelCall {
	return {
		kind: record.kind,
		promptTokens: record.promptTokens ?? 0,
		cachedPromptTokens: record.cachedPromptTokens ?? 0,
		completionTokens: record.completionTokens ?? 0,
		provider: record.upstreamProvider ?? '',
		latencyMs: record.latencyMs,
		providerReportedCostUSD: record.costUSD,
	};
}

function deliveredFileOf(record: GuestDeliveredFile): DeliveredFile {
	return {
		filename: record.filename ?? '',
		contentType: record.contentType ?? '',
		sizeBytes: record.sizeBytes ?? 0,
		devicePath: record.devicePath ?? '',
		isZipContainer: record.isZipContainer ?? null,
	};
}

function harnessStatusOf(guestStatus: string): HarnessStatus {
	if (guestStatus === 'completed') return 'completed';
	if (guestStatus === 'waiting_user_input') return 'waiting_user_input';
	return 'failed';
}

function unstartedOutcome(status: HarnessStatus, reply: string): HarnessOutcome {
	return { status, reachedTheLoop: false, turns: 0, toolCalls: [], reply, deliveredFiles: [], calls: [] };
}

export async function runBluecollar(context: ArmRunContext): Promise<HarnessOutcome> {
	const instructionBase64 = Buffer.from(context.instruction, 'utf8').toString('base64');
	const run = Bun.spawnSync(
		[
			join(context.repositoryRoot, 'internkim'),
			'lab',
			'vm-ssh',
			'--config',
			context.fleetConfigurationPath,
			'--',
			`printf '%s\\n' ${guestPassword} | sudo -S python3 ${guestScriptPath} ${context.requesterEmail} ${instructionBase64}`,
		],
		{ cwd: context.repositoryRoot, stdout: 'pipe', stderr: 'pipe', timeout: runTimeoutMs },
	);
	const stdout = new TextDecoder().decode(run.stdout);
	writeFileSync(join(context.evidenceDirectory, 'vm-ssh-stdout.txt'), stdout);
	writeFileSync(join(context.evidenceDirectory, 'vm-ssh-stderr.txt'), new TextDecoder().decode(run.stderr));
	const resultLine = stdout.split('\n').find((line) => line.startsWith(resultMarker));
	if (run.exitCode === null) return unstartedOutcome('timed_out', '');
	if (!resultLine) return unstartedOutcome('failed', `no result line; exit ${run.exitCode}`);
	const guest = JSON.parse(resultLine.slice(resultMarker.length)) as GuestResult;
	writeFileSync(join(context.evidenceDirectory, 'run-detail.json'), JSON.stringify(guest.detail, null, 2));
	return {
		status: harnessStatusOf(guest.status),
		reachedTheLoop: guest.turns > 0,
		turns: guest.turns,
		toolCalls: guest.requestedTools,
		reply: guest.status === 'completed' ? guest.result : guest.failureReason,
		deliveredFiles: (guest.deliveredFiles ?? []).map(deliveredFileOf),
		calls: guest.llmCalls.map(modelCallOf),
	};
}
