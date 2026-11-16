import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { agentDefaultNicknames } from '../../src/lib/persona/agent-names';

const personaSource = readFileSync(new URL('../../../internal/admind/persona.go', import.meta.url), 'utf8');

function goStringConstant(name: string): string | undefined {
	return personaSource.match(new RegExp(`\\b${name}\\s*=\\s*"([^"]+)"`))?.[1];
}

function admindDefaultNicknames(): string[] {
	const list = personaSource.match(/agentDefaultNicknames = \[\]string\{([^}]*)\}/)?.[1] ?? '';
	return list.split(',').map((entry) => entry.trim()).map((entry) => (entry.startsWith('"') ? entry.slice(1, -1) : goStringConstant(entry) ?? entry));
}

describe('the default nicknames are one list', () => {
	test('the settings screen protects the nicknames admind always keeps', () => {
		expect(agentDefaultNicknames).toEqual(admindDefaultNicknames());
	});
});
