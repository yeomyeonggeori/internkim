import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { aCompanyPlane } from './a-company-plane';

type ToolInventory = {
	tools: string[];
	quarantinedProviders: { providerID: string; reason: string }[];
};

type GeneratedCatalog = {
	tools: { modelName: string; answeredBy: string }[];
};

const generatedCatalogPath = join(
	import.meta.dir,
	'..',
	'..',
	'..',
	'pkg',
	'capabilityprotocol',
	'generated',
	'capability-tools.json'
);

function theToolsTheRecordAndTheCompanyAnswer(): string[] {
	const catalog = JSON.parse(readFileSync(generatedCatalogPath, 'utf8')) as GeneratedCatalog;
	return catalog.tools
		.filter((tool) => tool.answeredBy === 'company' || tool.answeredBy === 'record')
		.map((tool) => tool.modelName);
}

test('the agent on the plane registered every tool the catalog says the record or the company answers', async () => {
	const plane = await aCompanyPlane();
	try {
		const answer = await fetch(`${plane.blueclawURL}/admin/api/tools`);
		expect(answer.ok).toBe(true);
		const inventory = (await answer.json()) as ToolInventory;
		const registeredToolNames = new Set(inventory.tools);

		const missingToolNames = theToolsTheRecordAndTheCompanyAnswer().filter(
			(toolName) => !registeredToolNames.has(toolName)
		);

		expect(
			missingToolNames,
			`the plane never registered ${missingToolNames.join(', ')}. capabilityd's quarantined ` +
				`providers: ${JSON.stringify(inventory.quarantinedProviders)}`
		).toEqual([]);
	} finally {
		await plane.stop();
	}
}, 180_000);
