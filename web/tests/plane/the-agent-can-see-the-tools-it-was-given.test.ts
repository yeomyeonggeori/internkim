import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { aCompanyPlane } from './a-company-plane';

const repositoryRoot = join(import.meta.dir, '..', '..', '..');

type ToolInventory = {
	tools: string[];
	quarantinedProviders: { providerID: string; reason: string }[];
};

type CapabilityCatalog = {
	tools: { modelName: string; answeredBy: string }[];
};

function theToolsTheCatalogOffersThroughCapabilityd(): string[] {
	const catalogPath = join(repositoryRoot, 'pkg', 'capabilityprotocol', 'generated', 'capability-tools.json');
	const catalog = JSON.parse(readFileSync(catalogPath, 'utf8')) as CapabilityCatalog;
	return catalog.tools
		.filter((tool) => tool.answeredBy === 'company' || tool.answeredBy === 'record')
		.map((tool) => tool.modelName);
}

test('the agent on the plane registered every tool the catalog offers through capabilityd', async () => {
	const plane = await aCompanyPlane();
	try {
		const answer = await fetch(`${plane.blueclawURL}/admin/api/tools`);
		expect(answer.ok).toBe(true);
		const inventory = (await answer.json()) as ToolInventory;
		const registeredToolNames = new Set(inventory.tools);

		const missingToolNames = theToolsTheCatalogOffersThroughCapabilityd().filter(
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
