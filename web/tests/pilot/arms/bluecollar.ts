import { writeFileSync } from 'node:fs';
import { join } from 'node:path';
import type { ArmRunContext, HarnessOutcome, ModelCall } from './arm';

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

interface GuestResult {
	status: string;
	taskRunID: string;
	requestedTools: string[];
	llmCalls: LLMCallRecord[];
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
	if (run.exitCode === null) return { status: 'timed_out', turns: 0, toolCalls: [], reply: '', calls: [] };
	if (!resultLine) return { status: 'failed', turns: 0, toolCalls: [], reply: `no result line; exit ${run.exitCode}`, calls: [] };
	const guest = JSON.parse(resultLine.slice(resultMarker.length)) as GuestResult;
	writeFileSync(join(context.evidenceDirectory, 'run-detail.json'), JSON.stringify(guest.detail, null, 2));
	return {
		status: guest.status === 'completed' ? 'completed' : 'failed',
		turns: guest.turns,
		toolCalls: guest.requestedTools,
		reply: guest.status === 'completed' ? guest.result : guest.failureReason,
		calls: guest.llmCalls.map(modelCallOf),
	};
}
