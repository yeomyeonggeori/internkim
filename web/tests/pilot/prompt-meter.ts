import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import type { Subprocess } from 'bun';
import type { ModelCall } from './arms/arm';

export interface PromptMeter {
	port: number;
	ledgerPath: string;
	stop(): void;
}

const modelEndpoint = 'https://openrouter.ai/api/v1';

export function promptMeterPath(repositoryRoot: string): string {
	return join(repositoryRoot, '.dependency/blueclaw/.dependency/bluecollar/bench/terminalbench/prompt-meter');
}

async function untilListening(port: number): Promise<void> {
	for (let attempt = 0; attempt < 50; attempt += 1) {
		try {
			const socket = await Bun.connect({ hostname: '127.0.0.1', port, socket: { data() {} } });
			socket.end();
			return;
		} catch {
			await Bun.sleep(100);
		}
	}
	throw new Error(`the prompt meter did not start listening on ${port}`);
}

export async function startPromptMeter(repositoryRoot: string, harnessName: string, port: number, evidenceDirectory: string): Promise<PromptMeter> {
	const ledgerPath = join(evidenceDirectory, 'prompt-meter.jsonl');
	const meterProcess: Subprocess = Bun.spawn(['python3', promptMeterPath(repositoryRoot), 'serve', harnessName, String(port)], {
		env: {
			...process.env,
			BENCH_ENDPOINT: modelEndpoint,
			PROMPT_METER_LEDGER: ledgerPath,
			PROMPT_METER_BODY_DIRECTORY: join(evidenceDirectory, 'model-requests'),
		},
		stdout: 'ignore',
		stderr: 'pipe',
	});
	await untilListening(port);
	return { port, ledgerPath, stop: () => meterProcess.kill() };
}

interface LedgerLine {
	promptTokens: number | null;
	cachedPromptTokens: number | null;
	completionTokens: number | null;
	provider?: string | null;
	providerReportedCostUSD?: number | null;
	generationID?: string | null;
}

export function callsInLedger(ledgerPath: string): ModelCall[] {
	if (!existsSync(ledgerPath)) return [];
	return readFileSync(ledgerPath, 'utf8')
		.split('\n')
		.filter((line) => line.trim() !== '')
		.map((line) => JSON.parse(line) as LedgerLine)
		.map((line) => ({
			kind: 'chat',
			promptTokens: line.promptTokens ?? 0,
			cachedPromptTokens: line.cachedPromptTokens ?? 0,
			completionTokens: line.completionTokens ?? 0,
			provider: line.provider ?? '',
			providerReportedCostUSD: line.providerReportedCostUSD ?? undefined,
			generationID: line.generationID ?? undefined,
		}));
}

interface GenerationRecord {
	data?: { provider_name?: string; total_cost?: number };
}

// A Messages-shaped reply names no provider; the generation record does
// (https://openrouter.ai/docs/api-reference/get-a-generation), and it appears
// several seconds after a streamed reply ends.
async function generationRecord(generationID: string, modelCredential: string): Promise<GenerationRecord['data'] | undefined> {
	for (let attempt = 0; attempt < 8; attempt += 1) {
		const answer = await fetch(`${modelEndpoint}/generation?id=${encodeURIComponent(generationID)}`, {
			headers: { Authorization: `Bearer ${modelCredential}` },
		});
		if (answer.ok) return ((await answer.json()) as GenerationRecord).data;
		await answer.text();
		await Bun.sleep(3000);
	}
	return undefined;
}

export async function withProviders(calls: ModelCall[], modelCredential: string): Promise<ModelCall[]> {
	const resolved: ModelCall[] = [];
	for (const call of calls) {
		if (call.provider || !call.generationID) {
			resolved.push(call);
			continue;
		}
		const record = await generationRecord(call.generationID, modelCredential);
		resolved.push({
			...call,
			provider: record?.provider_name ?? '',
			providerReportedCostUSD: call.providerReportedCostUSD ?? record?.total_cost,
		});
	}
	return resolved;
}
