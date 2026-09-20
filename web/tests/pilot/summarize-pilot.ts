//   bun run tests/pilot/summarize-pilot.ts --run-id <id>

import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import { join, resolve } from 'node:path';
import type { ModelCall } from './arms/arm';
import type { RunResult } from './run-pilot';

const gateSampleSize = 30;

interface ArmSummary {
	arm: string;
	runs: number;
	passed: number;
	reachedTheLoop: number;
	passedInLoop: number;
	medianPromptTokensPerCall: number;
	medianCallsPerRun: number;
	totalPromptTokens: number;
	totalCachedPromptTokens: number;
	totalCompletionTokens: number;
	medianWallClockSeconds: number;
	costUSD: number;
	callsPricedByProvider: number;
	approvalsAnsweredByRequester: number;
	providers: Record<string, number>;
}

interface ListedPrice {
	promptUSDPerToken: number;
	completionUSDPerToken: number;
}

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

interface RunEvidence {
	result: RunResult;
	approvalsAnsweredByRequester: number;
}

interface LedgerEvent {
	name: string;
}

function approvalsAnsweredIn(runDirectory: string): number {
	const ledgerPath = join(runDirectory, 'run-detail.json');
	if (!existsSync(ledgerPath)) return 0;
	const ledger = JSON.parse(readFileSync(ledgerPath, 'utf8')) as { taskEvents: LedgerEvent[] };
	return ledger.taskEvents.filter((event) => event.name === 'approval.decided').length;
}

function evidenceUnder(directory: string): RunEvidence[] {
	if (!existsSync(directory)) return [];
	return readdirSync(directory).flatMap((name) => {
		const path = join(directory, name);
		if (statSync(path).isDirectory()) return evidenceUnder(path);
		if (name !== 'result.json') return [];
		return [{ result: JSON.parse(readFileSync(path, 'utf8')) as RunResult, approvalsAnsweredByRequester: approvalsAnsweredIn(directory) }];
	});
}

function median(values: number[]): number {
	if (values.length === 0) return 0;
	const sorted = [...values].sort((left, right) => left - right);
	const middle = Math.floor(sorted.length / 2);
	return sorted.length % 2 === 0 ? (sorted[middle - 1] + sorted[middle]) / 2 : sorted[middle];
}

async function listedPriceOf(model: string): Promise<ListedPrice> {
	const answer = await fetch('https://openrouter.ai/api/v1/models');
	if (!answer.ok) throw new Error(`the model list answered ${answer.status}`);
	const body = (await answer.json()) as { data: { id: string; pricing: { prompt: string; completion: string } }[] };
	const listed = body.data.find((entry) => entry.id === model);
	if (!listed) throw new Error(`${model} is not in the model list`);
	return { promptUSDPerToken: Number(listed.pricing.prompt), completionUSDPerToken: Number(listed.pricing.completion) };
}

// The provider reports what it actually charged, which is what a cache hit
// changes; list price says what the same tokens would have cost uncached.
function costOfCall(call: ModelCall, price: ListedPrice): number {
	if (call.providerReportedCostUSD !== undefined) return call.providerReportedCostUSD;
	return call.promptTokens * price.promptUSDPerToken + call.completionTokens * price.completionUSDPerToken;
}

function summarize(arm: string, evidence: RunEvidence[], price: ListedPrice): ArmSummary {
	const results = evidence.map((run) => run.result);
	const calls = results.flatMap((result) => result.calls);
	const providers: Record<string, number> = {};
	for (const call of calls) providers[call.provider || 'unknown'] = (providers[call.provider || 'unknown'] ?? 0) + 1;
	const promptTokens = calls.reduce((sum, call) => sum + call.promptTokens, 0);
	const completionTokens = calls.reduce((sum, call) => sum + call.completionTokens, 0);
	return {
		arm,
		runs: results.length,
		passed: results.filter((result) => result.passed).length,
		reachedTheLoop: results.filter((result) => result.reachedTheLoop).length,
		passedInLoop: results.filter((result) => result.reachedTheLoop && result.passed).length,
		medianPromptTokensPerCall: median(calls.map((call) => call.promptTokens)),
		medianCallsPerRun: median(results.map((result) => result.calls.length)),
		totalPromptTokens: promptTokens,
		totalCachedPromptTokens: calls.reduce((sum, call) => sum + call.cachedPromptTokens, 0),
		totalCompletionTokens: completionTokens,
		medianWallClockSeconds: median(results.map((result) => result.wallClockMs / 1000)),
		costUSD: calls.reduce((sum, call) => sum + costOfCall(call, price), 0),
		callsPricedByProvider: calls.filter((call) => call.providerReportedCostUSD !== undefined).length,
		approvalsAnsweredByRequester: evidence.reduce((sum, run) => sum + run.approvalsAnsweredByRequester, 0),
		providers,
	};
}

function percentage(part: number, whole: number): number {
	return whole === 0 ? 0 : (part / whole) * 100;
}

function row(summary: ArmSummary): string {
	const passRate = percentage(summary.passed, summary.runs);
	const inLoopPassRate = percentage(summary.passedInLoop, summary.reachedTheLoop);
	const costPerPassed = summary.passed === 0 ? '—' : `$${(summary.costUSD / summary.passed).toFixed(4)}`;
	const providers = Object.entries(summary.providers)
		.sort(([, left], [, right]) => right - left)
		.map(([name, count]) => `${name} ${count}`)
		.join(', ');
	const cachedShare = percentage(summary.totalCachedPromptTokens, summary.totalPromptTokens);
	return `| ${summary.arm} | ${summary.passed}/${summary.runs} (${passRate.toFixed(0)}%) | ${summary.reachedTheLoop}/${summary.runs} | ${summary.passedInLoop}/${summary.reachedTheLoop} (${inLoopPassRate.toFixed(0)}%) | ${summary.medianPromptTokensPerCall.toFixed(0)} | ${summary.medianCallsPerRun} | ${summary.totalPromptTokens.toLocaleString('en-US')} | ${cachedShare.toFixed(0)}% | ${summary.medianWallClockSeconds.toFixed(0)} | $${summary.costUSD.toFixed(4)} | ${costPerPassed} | ${summary.approvalsAnsweredByRequester} | ${providers} |`;
}

function gate(summaries: ArmSummary[], baselineArm: string, candidateArm: string): string {
	const baseline = summaries.find((summary) => summary.arm === baselineArm);
	const candidate = summaries.find((summary) => summary.arm === candidateArm);
	if (!baseline || !candidate) return `gate: not decidable, both ${baselineArm} and ${candidateArm} are needed`;
	const passRateHolds = candidate.passed / candidate.runs >= baseline.passed / baseline.runs;
	const tokensDrop = candidate.medianPromptTokensPerCall < baseline.medianPromptTokensPerCall;
	const verdict = passRateHolds && tokensDrop ? 'adopt' : 'reject';
	const sampleNote = Math.min(baseline.runs, candidate.runs) < gateSampleSize ? ` (pilot: fewer than ${gateSampleSize} samples per arm, not the final gate)` : '';
	const inLoop = `in-loop pass rate ${percentage(candidate.passedInLoop, candidate.reachedTheLoop).toFixed(0)}% against ${percentage(baseline.passedInLoop, baseline.reachedTheLoop).toFixed(0)}%, over ${candidate.reachedTheLoop} and ${baseline.reachedTheLoop} runs that reached the loop`;
	return `gate ${candidateArm} against ${baselineArm}: ${verdict} — pass rate ${passRateHolds ? 'holds' : 'drops'}, median prompt tokens per call ${tokensDrop ? 'lower' : 'not lower'}${sampleNote}; ${inLoop}`;
}

async function main(): Promise<void> {
	const runID = argument('run-id');
	if (!runID) throw new Error('pass --run-id <id>');
	const baselineArm = argument('baseline') ?? 'bluecollar';
	const candidateArm = argument('candidate') ?? 'bluecollar-pi-shaped';
	const evidenceRoot = resolve(import.meta.dir, '../../../.artifacts/pilot', runID);
	const evidence = evidenceUnder(evidenceRoot);
	const results = evidence.map((run) => run.result);
	if (results.length === 0) throw new Error(`no result.json under ${evidenceRoot}`);
	const models = [...new Set(results.map((result) => result.model))];
	if (models.length !== 1) throw new Error(`one model per pilot; found ${models.join(', ')}`);
	const price = await listedPriceOf(models[0]);
	const arms = [...new Set(results.map((result) => result.arm))].sort();
	const summaries = arms.map((arm) => summarize(arm, evidence.filter((run) => run.result.arm === arm), price));

	const callCount = results.reduce((sum, result) => sum + result.calls.length, 0);
	const pricedByProvider = summaries.reduce((sum, summary) => sum + summary.callsPricedByProvider, 0);
	console.log(`pilot ${runID} · model ${models[0]} · listed price $${price.promptUSDPerToken * 1e6}/M prompt, $${price.completionUSDPerToken * 1e6}/M completion · ${pricedByProvider}/${callCount} calls priced from the provider's own report\n`);
	console.log('| arm | passed | reached the loop | passed in the loop | median prompt tokens/call | median calls/run | total prompt tokens | cached share | median wall clock s | cost | cost per passed task | approvals answered | providers |');
	console.log('|---|---|---|---|---|---|---|---|---|---|---|---|---|');
	for (const summary of summaries) console.log(row(summary));
	console.log(`\n${gate(summaries, baselineArm, candidateArm)}`);
	const failed = results.filter((result) => !result.passed);
	if (failed.length > 0) {
		console.log('\nfailed runs:');
		for (const result of failed) {
			const reasons = result.judgement.findings.flatMap((finding) => finding.mismatches).join('; ');
			const loop = result.reachedTheLoop ? 'in the loop' : 'never reached the loop';
			console.log(`- ${result.arm} ${result.task} #${result.repetition}: ${result.harness.status}, ${loop}; ${reasons}`);
		}
	}
}

await main();
