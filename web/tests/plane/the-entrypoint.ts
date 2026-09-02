import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const entrypointPath = join(import.meta.dir, '..', '..', '..', 'host', 'entrypoint.sh');

type AFlag = { name: string; value: string };

function theEntrypoint(): string {
	return readFileSync(entrypointPath, 'utf8');
}

function theCommandThatStarts(program: string): string {
	const lines = theEntrypoint().split('\n');
	const firstLine = lines.findIndex((line) => line.trimStart().startsWith(`${program} `));
	if (firstLine === -1) throw new Error(`host/entrypoint.sh starts no ${program}`);
	const command: string[] = [];
	for (let line = firstLine; line < lines.length; line += 1) {
		const text = lines[line].trimEnd();
		command.push(text.endsWith('\\') ? text.slice(0, -1) : text);
		if (!text.endsWith('\\')) break;
	}
	return command.join(' ');
}

function unquoted(token: string): string {
	return token.replace(/^["']|["']$/g, '');
}

function isFlag(token: string): boolean {
	return /^-{1,2}[A-Za-z][A-Za-z0-9-]*$/.test(token);
}

function theFlagsThatStart(program: string): AFlag[] {
	const tokens = theCommandThatStarts(program).trim().split(/\s+/).slice(1);
	const flags: AFlag[] = [];
	for (let index = 0; index < tokens.length; index += 1) {
		if (!isFlag(tokens[index])) continue;
		const next = tokens[index + 1];
		flags.push({ name: tokens[index], value: next && !isFlag(next) ? unquoted(next) : '' });
	}
	return flags;
}

export function whatStarts(program: string, flagName: string): string {
	const bare = flagName.replace(/^-+/, '');
	const found = theFlagsThatStart(program).find((flag) => flag.name.replace(/^-+/, '') === bare);
	if (!found) throw new Error(`host/entrypoint.sh starts ${program} without ${flagName}`);
	return found.value;
}

export function theArgumentsThatStart(program: string, values: Record<string, string>): string[] {
	return theFlagsThatStart(program).flatMap(({ name }) => {
		const value = values[name];
		if (value === undefined) {
			throw new Error(
				`host/entrypoint.sh starts ${program} with ${name} and this sandbox has no value for it, ` +
					`so the plane it brings up is not the plane a company runs`
			);
		}
		return [name, value];
	});
}

export function theProgramsTheEntrypointRuns(): string[] {
	const declared = theEntrypoint().match(/programsThisScriptRuns="([^"]*)"/);
	if (!declared) throw new Error('host/entrypoint.sh no longer declares programsThisScriptRuns');
	return declared[1].split(/\s+/).filter(Boolean);
}
