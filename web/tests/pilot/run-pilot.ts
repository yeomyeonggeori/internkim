//   monkeys run @test bun run tests/pilot/run-pilot.ts --arm <bluecollar|bluecollar-pi-shaped|bluecollar-reply-action|claude-code> [--repetitions 3] [--tasks tests/pilot/tasks] [--run-id <id>] [--only <task name>]
//
//   PILOT_APP_URL          the local app the external harness reaches over MCP (claude-code)
//   PILOT_FLEET_CONFIG     the kept Local Fleet's config.json (bluecollar arms)
//   OPENROUTER_API_KEY     the model credential an external harness sends through the meter (claude-code)

import { existsSync, mkdirSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import type { SupabaseClient } from '@supabase/supabase-js';
import { controlPlane, forgetPersonalAccessToken, issuePersonalAccessToken } from '../../src/lib/server/control-plane';
import { memberOfCompanyByEmail } from '../../src/lib/server/member-credential';
import { armNames, isArmName, type ArmName, type ArmRunContext, type HarnessOutcome } from './arms/arm';
import { runBluecollar } from './arms/bluecollar';
import { runClaudeCode } from './arms/claude-code';
import { cleanUp, judge, loadTasks, localRecord, seedCompanyID, type Judgement, type PilotTask } from './record';

const model = 'z-ai/glm-5.3-flash';
const defaultRequesterEmail = 'member3@example.com';

interface Options {
	arm: ArmName;
	repetitions: number;
	tasksDirectory: string;
	runID: string;
	only?: string;
}

export interface RunResult {
	arm: ArmName;
	task: string;
	requesterEmail: string;
	repetition: number;
	model: string;
	startedAt: string;
	wallClockMs: number;
	passed: boolean;
	reachedTheLoop: boolean;
	judgement: Judgement;
	harness: Omit<HarnessOutcome, 'calls' | 'reachedTheLoop'>;
	calls: HarnessOutcome['calls'];
}

interface RequesterCredential {
	memberID: string;
	tokenName: string;
	personalAccessToken: string;
}

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

function options(): Options {
	const arm = argument('arm') ?? '';
	if (!isArmName(arm)) throw new Error(`--arm is one of ${armNames.join(', ')}`);
	return {
		arm,
		repetitions: Number(argument('repetitions') ?? 3),
		tasksDirectory: resolve(argument('tasks') ?? join(import.meta.dir, 'tasks')),
		runID: argument('run-id') ?? new Date().toISOString().replace(/[-:]/g, '').replace(/\..*/, '').toLowerCase(),
		only: argument('only'),
	};
}

function requiredEnvironment(name: string, arm: ArmName): string {
	const value = (process.env[name] ?? '').trim();
	if (!value) throw new Error(`${name} must be set for the ${arm} arm`);
	return value;
}

function armRunner(arm: ArmName): (context: ArmRunContext) => Promise<HarnessOutcome> {
	return arm === 'claude-code' ? runClaudeCode : runBluecollar;
}

function tokenSum(calls: HarnessOutcome['calls']): number {
	return calls.reduce((sum, call) => sum + call.promptTokens, 0);
}

function requesterOf(task: PilotTask): string {
	return task.requesterEmail ?? defaultRequesterEmail;
}

async function issueCredential(client: SupabaseClient, email: string, runID: string): Promise<RequesterCredential> {
	const memberID = await memberOfCompanyByEmail(client, seedCompanyID, email);
	if (!memberID) throw new Error(`${email} is not a member of the seed company; reset the local record first`);
	const tokenName = `pilot-${runID}-${email.split('@')[0]}`;
	const personalAccessToken = await issuePersonalAccessToken(client, memberID, tokenName, 'write');
	return { memberID, tokenName, personalAccessToken };
}

async function credentialFor(
	client: SupabaseClient,
	credentials: Map<string, RequesterCredential>,
	email: string,
	runID: string,
): Promise<RequesterCredential> {
	const known = credentials.get(email);
	if (known) return known;
	const issued = await issueCredential(client, email, runID);
	credentials.set(email, issued);
	return issued;
}

async function forgetCredentials(client: SupabaseClient, credentials: Map<string, RequesterCredential>): Promise<void> {
	for (const credential of credentials.values()) {
		await forgetPersonalAccessToken(client, credential.memberID, credential.tokenName);
	}
}

async function main(): Promise<void> {
	const chosen = options();
	const repositoryRoot = resolve(import.meta.dir, '../../..');
	const record = localRecord();
	const tasks = loadTasks(chosen.tasksDirectory, chosen.only);
	if (tasks.length === 0) throw new Error(`no task in ${chosen.tasksDirectory}`);
	const evidenceRoot = join(repositoryRoot, '.artifacts/pilot', chosen.runID, chosen.arm);
	const isExternalHarness = chosen.arm === 'claude-code';

	const client = controlPlane({ projectURL: record.apiURL, serviceRoleKey: record.secretKey });
	const credentials = new Map<string, RequesterCredential>();

	const context: Omit<ArmRunContext, 'instruction' | 'evidenceDirectory' | 'personalAccessToken' | 'requesterEmail'> = {
		repositoryRoot,
		appURL: isExternalHarness ? requiredEnvironment('PILOT_APP_URL', chosen.arm) : '',
		modelCredential: isExternalHarness ? requiredEnvironment('OPENROUTER_API_KEY', chosen.arm) : '',
		model,
		fleetConfigurationPath: isExternalHarness ? '' : resolve(requiredEnvironment('PILOT_FLEET_CONFIG', chosen.arm)),
	};

	console.log(`pilot ${chosen.runID}: arm ${chosen.arm}, ${tasks.length} task(s) × ${chosen.repetitions}, evidence under ${evidenceRoot}`);
	try {
		for (let repetition = 1; repetition <= chosen.repetitions; repetition += 1) {
			for (const task of tasks) {
				const evidenceDirectory = join(evidenceRoot, task.name, String(repetition));
				if (existsSync(join(evidenceDirectory, 'result.json'))) continue;
				mkdirSync(evidenceDirectory, { recursive: true });
				const requesterEmail = requesterOf(task);
				const credential = await credentialFor(client, credentials, requesterEmail, chosen.runID);
				await cleanUp(record, task.cleanup);
				const startedAt = new Date();
				const outcome = await armRunner(chosen.arm)({
					...context,
					requesterEmail,
					personalAccessToken: credential.personalAccessToken,
					instruction: task.instruction,
					evidenceDirectory,
				});
				const wallClockMs = Date.now() - startedAt.getTime();
				const judgement = await judge(record, task.assertions, outcome);
				await cleanUp(record, task.cleanup);
				const { calls, reachedTheLoop, ...harness } = outcome;
				const result: RunResult = {
					arm: chosen.arm,
					task: task.name,
					requesterEmail,
					repetition,
					model,
					startedAt: startedAt.toISOString(),
					wallClockMs,
					passed: judgement.passed,
					reachedTheLoop,
					judgement,
					harness,
					calls,
				};
				writeFileSync(join(evidenceDirectory, 'result.json'), JSON.stringify(result, null, 2));
				const verdict = judgement.passed ? '✓' : '✗';
				const loop = reachedTheLoop ? 'in the loop' : 'never reached the loop';
				const reason = judgement.passed ? '' : ` — ${judgement.findings.flatMap((finding) => finding.mismatches).join('; ')}`;
				console.log(
					`${verdict} ${chosen.arm} ${task.name} #${repetition}: ${harness.status}, ${loop}, ${calls.length} model calls, ${tokenSum(calls)} prompt tokens, ${(wallClockMs / 1000).toFixed(0)} s${reason}`,
				);
			}
		}
	} finally {
		await forgetCredentials(client, credentials);
	}
}

await main();
