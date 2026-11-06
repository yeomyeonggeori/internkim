//   bun run tests/e2e/judge-delivery.ts <case directory>

import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import type { DeliveredFile, HarnessStatus } from '../pilot/arms/arm';
import { cleanUp, judge, localRecord, type JudgedOutcome, type PilotTask } from '../pilot/record';

interface KeptFile {
	name: string;
	m?: string;
	size?: string;
	fetched?: boolean;
	workspacePaths?: string[];
}

interface KeptDelivery {
	outcome: string;
	replies?: { content?: string }[];
	files?: KeptFile[];
	inboxes?: Record<string, string[]>;
}

const zipSignature = 'PK';

function statusOf(outcome: string): HarnessStatus {
	if (outcome === 'completed' || outcome === 'waiting_user_input') return outcome;
	if (outcome === 'running' || outcome === 'planned') return 'timed_out';
	return 'failed';
}

function isZipContainer(caseDirectory: string, file: KeptFile): boolean | null {
	const path = join(caseDirectory, file.name);
	if (!file.fetched || !existsSync(path)) return null;
	return readFileSync(path).subarray(0, 2).toString('latin1') === zipSignature;
}

function deliveredFileOf(caseDirectory: string, file: KeptFile): DeliveredFile {
	return {
		filename: file.name,
		contentType: file.m ?? '',
		sizeBytes: Number(file.size ?? 0),
		devicePath: file.workspacePaths?.[0] ?? '',
		isZipContainer: isZipContainer(caseDirectory, file),
	};
}

function outcomeOf(caseDirectory: string, delivery: KeptDelivery): JudgedOutcome {
	return {
		status: statusOf(delivery.outcome),
		reply: (delivery.replies ?? []).map((reply) => reply.content ?? '').join('\n'),
		deliveredFiles: (delivery.files ?? []).map((file) => deliveredFileOf(caseDirectory, file)),
		inboxes: delivery.inboxes ?? {},
	};
}

function readJSON<T>(path: string): T {
	return JSON.parse(readFileSync(path, 'utf8')) as T;
}

async function main(): Promise<void> {
	const caseDirectory = process.argv[2];
	if (!caseDirectory) throw new Error('usage: judge-delivery.ts <case directory>');
	const task = readJSON<PilotTask>(join(caseDirectory, 'task.json'));
	const delivery = readJSON<KeptDelivery>(join(caseDirectory, 'result.json'));
	const record = localRecord();
	const judgement = await judge(record, task.assertions, outcomeOf(caseDirectory, delivery));
	await cleanUp(record, task.cleanup);
	writeFileSync(join(caseDirectory, 'judgement.json'), JSON.stringify(judgement, null, 2));
	console.log(JSON.stringify(judgement));
}

await main();
