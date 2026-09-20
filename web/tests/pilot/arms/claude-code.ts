import { mkdtempSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { extname, join } from 'node:path';
import { callsInLedger, startPromptMeter, withProviders } from '../prompt-meter';
import type { ArmRunContext, DeliveredFile, HarnessOutcome } from './arm';

const meterPort = 8331;
const serverName = 'internkim-plane';
const maximumTurns = 12;
const runTimeoutMs = 600_000;

const contentTypeByExtension: Record<string, string> = {
	'.csv': 'text/csv',
	'.docx': 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
	'.html': 'text/html',
	'.md': 'text/markdown',
	'.pdf': 'application/pdf',
	'.pptx': 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
	'.txt': 'text/plain',
	'.xlsx': 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
};

const zipLocalFileHeader = Buffer.from([0x50, 0x4b, 0x03, 0x04]);

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

function startsWithZipHeader(path: string): boolean | null {
	try {
		return readFileSync(path).subarray(0, zipLocalFileHeader.length).equals(zipLocalFileHeader);
	} catch {
		return null;
	}
}

function deliveredFilesIn(directory: string): DeliveredFile[] {
	return readdirSync(directory, { withFileTypes: true })
		.filter((entry) => entry.isFile())
		.map((entry) => {
			const path = join(directory, entry.name);
			const extension = extname(entry.name).toLowerCase();
			return {
				filename: entry.name,
				contentType: contentTypeByExtension[extension] ?? 'application/octet-stream',
				sizeBytes: statSync(path).size,
				devicePath: path,
				isZipContainer: startsWithZipHeader(path),
			};
		});
}

function outcomeOf(stdout: string, exitCode: number | null, deliveredFiles: DeliveredFile[], calls: HarnessOutcome['calls']): HarnessOutcome {
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
	return {
		status,
		reachedTheLoop: lines.some((line) => line.type === 'assistant'),
		turns: result?.num_turns ?? 0,
		toolCalls,
		reply: result?.result ?? '',
		deliveredFiles,
		calls,
	};
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
		return outcomeOf(stdout, run.exitCode, deliveredFilesIn(workingDirectory), calls);
	} finally {
		meter.stop();
		rmSync(privateDirectory, { recursive: true, force: true });
	}
}
