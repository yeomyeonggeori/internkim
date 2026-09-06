import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const entrypointPath = join(import.meta.dir, '..', '..', '..', 'host', 'entrypoint.sh');

type AFlag = { name: string; rawValue: string; isJoined?: boolean };
type AToken = { text: string; isOperator: boolean; isRedirect: boolean };

const shellWords = new Set([
	'if', 'then', 'elif', 'else', 'fi', 'for', 'in', 'do', 'done', 'while', 'until', 'case', 'esac',
	'set', 'exit', 'echo', 'printf', 'trap', 'wait', 'kill', 'sleep', 'true', 'false', 'cd', 'export',
	'command', 'return', 'shift', 'read', 'eval', 'exec', 'local', 'unset', 'test', ':', '[', '{',
	'}', '!', 'umask'
]);

const introducesACommand = new Set(['if', 'then', 'elif', 'else', 'do', 'while', 'until', '!']);

const assignmentPrefix = /^[A-Za-z_][A-Za-z0-9_]*=/;
const flagName = /^--?[A-Za-z][A-Za-z0-9-]*$/;
const flagWithJoinedValue = /^--?[A-Za-z][A-Za-z0-9-]*=/;

function theEntrypoint(): string {
	return readFileSync(entrypointPath, 'utf8');
}

function refuse(what: string): never {
	throw new Error(`host/entrypoint.sh ${what}, and this reader will not guess what sh would do`);
}

function theLogicalLines(): string[] {
	const joined: string[] = [];
	let pending = '';
	for (const [index, line] of theEntrypoint().split('\n').entries()) {
		if (/\\[ \t]+$/.test(line)) {
			refuse(`ends line ${index + 1} with a backslash and trailing space, which continues nothing`);
		}
		const continues = line.endsWith('\\');
		pending += (pending ? ' ' : '') + (continues ? line.slice(0, -1) : line);
		if (continues) continue;
		if (pending.trim()) joined.push(pending.trim());
		pending = '';
	}
	return joined;
}

function wholeWordAt(logicalLine: string, start: number): string {
	const rest = logicalLine.slice(start);
	return rest.slice(0, rest.search(/\s|$/));
}

function tokenize(logicalLine: string): AToken[] {
	const tokens: AToken[] = [];
	let current = '';
	let quote = '';
	let braceDepth = 0;
	const pushWord = () => {
		if (current) tokens.push({ text: current, isOperator: false, isRedirect: false });
		current = '';
	};
	for (let index = 0; index < logicalLine.length; index += 1) {
		const character = logicalLine[index];
		if (quote) {
			current += character;
			if (character === quote) quote = '';
			continue;
		}
		if (braceDepth > 0) {
			current += character;
			if (character === '{') braceDepth += 1;
			if (character === '}') braceDepth -= 1;
			continue;
		}
		if (character === '"' || character === "'") {
			quote = character;
			current += character;
			continue;
		}
		if (character === '$' && logicalLine[index + 1] === '{') {
			braceDepth = 1;
			current += '${';
			index += 1;
			continue;
		}
		if (character === '#' && current === '') return tokens;
		if (/\s/.test(character)) {
			pushWord();
			continue;
		}
		if (character === '>' || character === '<') {
			if (/^\d*$/.test(current)) current = '';
			pushWord();
			let redirection = '';
			while (index < logicalLine.length && !/\s/.test(logicalLine[index])) {
				redirection += logicalLine[index];
				index += 1;
			}
			tokens.push({ text: redirection, isOperator: true, isRedirect: true });
			continue;
		}
		const pair = logicalLine.slice(index, index + 2);
		if (pair === '&&' || pair === '||') {
			pushWord();
			tokens.push({ text: pair, isOperator: true, isRedirect: false });
			index += 1;
			continue;
		}
		if ('();|&'.includes(character)) {
			pushWord();
			tokens.push({ text: character, isOperator: true, isRedirect: false });
			continue;
		}
		current += character;
	}
	pushWord();
	return tokens;
}

function theFunctionsDefinedHere(): Set<string> {
	return new Set(
		theLogicalLines()
			.map((logicalLine) => logicalLine.match(/^([A-Za-z_][A-Za-z0-9_]*)\s*\(\)/)?.[1])
			.filter((name): name is string => Boolean(name))
	);
}

function theCommandWordsOf(logicalLine: string): string[] {
	const tokens = tokenize(logicalLine);
	const words: string[] = [];
	let atCommandStart = true;
	for (let index = 0; index < tokens.length; index += 1) {
		const token = tokens[index];
		if (token.isRedirect) {
			if (/^\d*[<>]{1,2}$/.test(token.text)) index += 1;
			continue;
		}
		if (token.isOperator) {
			atCommandStart = true;
			continue;
		}
		if (!atCommandStart) continue;
		if (assignmentPrefix.test(token.text)) continue;
		words.push(token.text);
		atCommandStart = introducesACommand.has(token.text);
	}
	return words;
}

export function theCommandWordsTheEntrypointRuns(): string[] {
	const defined = theFunctionsDefinedHere();
	const words = theLogicalLines().flatMap(theCommandWordsOf);
	return [
		...new Set(
			words.filter(
				(word) =>
					!shellWords.has(word) &&
					!defined.has(word) &&
					!/^["'$]/.test(word) &&
					!assignmentPrefix.test(word)
			)
		)
	].sort();
}

function theCommandThatStarts(program: string): AToken[] {
	const found = theLogicalLines().find((logicalLine) => {
		if (theCommandWordsOf(logicalLine)[0] === program) return true;
		const tokens = tokenize(logicalLine);
		const separator = tokens.findIndex((token) => token.text === '--');
		return theCommandWordsOf(logicalLine)[0] === 'runuser' && tokens[separator + 1]?.text === program;
	});
	if (!found) throw new Error(`host/entrypoint.sh starts no ${program}`);
	const tokens = tokenize(found);
	const separator = tokens.findIndex((token) => token.text === '--');
	const start = theCommandWordsOf(found)[0] === 'runuser'
		? separator + 1
		: tokens.findIndex((token) => token.text === program && !token.isOperator);
	return tokens.slice(start + 1);
}

function unquoted(token: string): string {
	const opening = token[0];
	if (opening !== '"' && opening !== "'") return token;
	if (token.length < 2 || !token.endsWith(opening)) {
		refuse(`opens a value with ${opening} and closes it somewhere this reader cannot see`);
	}
	return token.slice(1, -1);
}

function withScriptVariablesResolved(token: string, program: string): string {
	return unquoted(token).replace(/\$\{([A-Za-z_][A-Za-z0-9_]*)\}/g, (whole, name: string) => {
		const assignments = theLogicalLines().filter((logicalLine) =>
			logicalLine.startsWith(`${name}=`)
		);
		if (assignments.length > 1) {
			refuse(
				`assigns ${name} ${assignments.length} times, so what ${program} is started with depends on which one ran`
			);
		}
		if (assignments.length === 0) return whole;
		return withScriptVariablesResolved(assignments[0].slice(name.length + 1), program);
	});
}

function theFlagsThatStart(program: string): AFlag[] {
	const tokens = theCommandThatStarts(program);
	const flags: AFlag[] = [];
	for (let index = 0; index < tokens.length; index += 1) {
		const token = tokens[index];
		if (token.isRedirect) refuse(`starts ${program} with a redirection this reader cannot represent`);
		if (token.isOperator) {
			if (token.text === '&') continue;
			refuse(`starts ${program} through ${token.text}, which this reader cannot represent`);
		}
		if (flagWithJoinedValue.test(token.text)) {
			const separator = token.text.indexOf('=');
			flags.push({ name: token.text.slice(0, separator), rawValue: token.text.slice(separator + 1), isJoined: true });
			continue;
		}
		if (!flagName.test(token.text)) {
			refuse(`starts ${program} with a bare ${token.text} that follows no flag`);
		}
		const next = tokens[index + 1];
		if (!next || next.isOperator || flagName.test(next.text)) {
			refuse(`starts ${program} with ${token.text} and no value`);
		}
		flags.push({ name: token.text, rawValue: next.text });
		index += 1;
	}
	return flags;
}

export function whatStarts(program: string, name: string): string {
	const bare = name.replace(/^-+/, '');
	const found = theFlagsThatStart(program).find((flag) => flag.name.replace(/^-+/, '') === bare);
	if (!found) throw new Error(`host/entrypoint.sh starts ${program} without ${name}`);
	return withScriptVariablesResolved(found.rawValue, program);
}

export function theArgumentsThatStart(
	program: string,
	shared: Record<string, string>,
	sandboxOnly: Record<string, string> = {}
): string[] {
	const flags = theFlagsThatStart(program);
	const started = flags.map((flag) => flag.name);
	for (const name of Object.keys(shared)) {
		if (started.includes(name)) continue;
		throw new Error(
			`this sandbox starts ${program} with ${name} as if host/entrypoint.sh did, and it does not`
		);
	}
	for (const name of Object.keys(sandboxOnly)) {
		if (!started.includes(name)) continue;
		throw new Error(
			`host/entrypoint.sh now starts ${program} with ${name}, so it is no longer this sandbox's own: ` +
				`give it the entrypoint's value rather than a sandbox-only one`
		);
	}
	const fromTheEntrypoint = started.flatMap((name) => {
		const value = shared[name];
		if (value === undefined) {
			throw new Error(
				`host/entrypoint.sh starts ${program} with ${name} and this sandbox has no value for it, ` +
					`so the plane it brings up is not the plane a company runs`
			);
		}
		return flags.find((flag) => flag.name === name)?.isJoined ? [`${name}=${value}`] : [name, value];
	});
	return [...fromTheEntrypoint, ...Object.entries(sandboxOnly).flat()];
}

export function theProgramsTheEntrypointDeclares(): string[] {
	const declared = theEntrypoint().match(/programsThisScriptRuns="([^"]*)"/);
	if (!declared) throw new Error('host/entrypoint.sh no longer declares programsThisScriptRuns');
	return declared[1].split(/\s+/).filter(Boolean);
}
