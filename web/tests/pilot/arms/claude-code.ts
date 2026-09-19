import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { callsInLedger, startPromptMeter, withProviders } from '../prompt-meter';
import type { ArmRunContext, HarnessOutcome } from './arm';

const meterPort = 8331;
const serverName = 'internkim-plane';
const maximumTurns = 12;
const runTimeoutMs = 600_000;

interface StreamLine {
	type: string;
	subtype?: string;
	is_error?: boolean;
	num_turns?: number;
	result?: string;
	message?: { content?: { type: string; name?: string }[] };
}

function mcpConfiguration(appURL: string, personalAccessToken: string): string {
	return JSON.stringify({
		mcpServers: {
			[serverName]: {
				type: 'http',
				url: `${appURL}/api/v1/mcp`,
				headers: { Authorization: `Bearer ${personalAccessToken}` },
			},
		},
	});
}

function outcomeOf(stdout: string, exitCode: number | null, calls: HarnessOutcome['calls']): HarnessOutcome {
	const lines = stdout
		.split('\n')
		.filter((line) => line.startsWith('{'))
		.map((line) => JSON.parse(line) as StreamLine);
	const toolCalls = lines
		.filter((line) => line.type === 'assistant')
		.flatMap((line) => line.message?.content ?? [])
		.filter((block) => block.type === 'tool_use')
		.map((block) => block.name ?? '');
	const result = lines.find((line) => line.type === 'result');
	const status = exitCode === null ? 'timed_out' : result && !result.is_error && result.subtype === 'success' ? 'completed' : 'failed';
	return { status, turns: result?.num_turns ?? 0, approvalsAnsweredByRequester: 0, toolCalls, reply: result?.result ?? '', calls };
}

export async function runClaudeCode(context: ArmRunContext): Promise<HarnessOutcome> {
	const meter = await startPromptMeter(context.repositoryRoot, 'claude-code', meterPort, context.evidenceDirectory);
	const privateDirectory = mkdtempSync(join(tmpdir(), 'pilot-claude-'));
	const configurationPath = join(privateDirectory, 'mcp.json');
	const workingDirectory = join(privateDirectory, 'cwd');
	writeFileSync(configurationPath, mcpConfiguration(context.appURL, context.personalAccessToken), { mode: 0o600 });
	Bun.spawnSync(['mkdir', '-p', workingDirectory]);
	try {
		const run = Bun.spawnSync(
			[
				'claude',
				'-p',
				context.instruction,
				'--output-format',
				'stream-json',
				'--verbose',
				'--mcp-config',
				configurationPath,
				'--strict-mcp-config',
				'--setting-sources',
				'',
				'--allowedTools',
				`mcp__${serverName}__*`,
				'--max-turns',
				String(maximumTurns),
				'--model',
				context.model,
			],
			{
				cwd: workingDirectory,
				env: {
					...process.env,
					CLAUDECODE: '',
					ANTHROPIC_API_KEY: '',
					ANTHROPIC_BASE_URL: `http://127.0.0.1:${meterPort}/api`,
					ANTHROPIC_AUTH_TOKEN: context.modelCredential,
					ANTHROPIC_MODEL: context.model,
				},
				stdout: 'pipe',
				stderr: 'pipe',
				timeout: runTimeoutMs,
			},
		);
		const stdout = new TextDecoder().decode(run.stdout);
		writeFileSync(join(context.evidenceDirectory, 'claude-stream.jsonl'), stdout);
		writeFileSync(join(context.evidenceDirectory, 'claude-stderr.txt'), new TextDecoder().decode(run.stderr));
		const calls = await withProviders(callsInLedger(meter.ledgerPath), context.modelCredential);
		return outcomeOf(stdout, run.exitCode, calls);
	} finally {
		meter.stop();
		rmSync(privateDirectory, { recursive: true, force: true });
	}
}
