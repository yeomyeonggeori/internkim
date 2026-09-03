import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { aCompanyPlane } from './a-company-plane';

type ToolInventory = {
	tools: string[];
	quarantinedProviders: { providerID: string; reason: string }[];
};

type CapabilitiesDocument = {
	toolDescriptors: { modelName: string; answeredBy: string }[];
};

function theToolsCapabilitydOffersFor(capabilitiesDocumentPath: string): string[] {
	const capabilities = JSON.parse(readFileSync(capabilitiesDocumentPath, 'utf8')) as CapabilitiesDocument;
	return capabilities.toolDescriptors
		.filter((toolDescriptor) => toolDescriptor.answeredBy === 'company' || toolDescriptor.answeredBy === 'record')
		.map((toolDescriptor) => toolDescriptor.modelName);
}

test('the agent on the plane registered every tool capabilityd offered it', async () => {
	const plane = await aCompanyPlane();
	try {
		const answer = await fetch(`${plane.blueclawURL}/admin/api/tools`);
		expect(answer.ok).toBe(true);
		const inventory = (await answer.json()) as ToolInventory;
		const registeredToolNames = new Set(inventory.tools);

		const missingToolNames = theToolsCapabilitydOffersFor(plane.capabilitiesDocumentPath).filter(
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
