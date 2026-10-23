import { readFileSync } from 'node:fs';
import { describe, expect, test } from 'bun:test';
import { crmStageKeys, crmStageOutcomes } from '../../../src/lib/crm/crm-stage';
import { crmStages } from '../../../src/routes/crm/crm-stages';
import { buildCapabilityToolCatalog } from '../../../src/lib/server/public-api/catalog/tools';
import { protocolVersion } from '../../../src/lib/server/public-api/catalog/protocol';

function stagesTheDatabaseIsPinnedTo(): string[] {
	const pinned = readFileSync('../supabase/tests/a_stage_is_one_of_six_words.test.sql', 'utf8');
	const declaration = pinned.match(/array\[([^\]]+)\]/);
	if (!declaration) throw new Error('the pgTAP test no longer pins the stage enum to an array literal');
	return declaration[1]
		.split(',')
		.map((value) => value.trim().replace(/^'|'$/g, ''))
		.sort();
}

function stagesATakes(name: string, field: string): string[] {
	const catalog = buildCapabilityToolCatalog(protocolVersion);
	const descriptor = catalog.tools.find((tool) => tool.name === name);
	if (!descriptor) throw new Error(`${name} is not in the catalog`);
	const schema = descriptor.inputSchema as { properties?: Record<string, { enum?: string[] }> };
	const taken = schema.properties?.[field]?.enum;
	if (!taken) throw new Error(`${name} takes no ${field} enum`);
	return [...taken].sort();
}

describe('a stage is one of six words, declared once', () => {
	test('the database enum names the same six', () => {
		expect(stagesTheDatabaseIsPinnedTo()).toEqual([...crmStageKeys].sort());
	});

	test('the catalog takes the six the record knows', () => {
		expect(stagesATakes('crm_opportunity_move', 'stage')).toEqual([...crmStageKeys].sort());
		expect(stagesATakes('crm_opportunity_list', 'stage')).toEqual([...crmStageKeys].sort());
	});

	test('the CRM screens read the same six, in the order the boards draw them', () => {
		expect(crmStages.map((entry) => entry.stage)).toEqual([...crmStageKeys]);
		expect(crmStages.map((entry) => entry.position)).toEqual([1, 2, 3, 4, 5, 6]);
		expect(crmStages.map((entry) => entry.outcome)).toEqual(
			crmStageKeys.map((stage) => crmStageOutcomes[stage])
		);
	});
});
