import { describe, expect, test } from 'bun:test';

import { capabilityNamespaceSummaries } from '../../../src/lib/server/public-api/catalog/namespaces';
import { protocolVersion } from '../../../src/lib/server/public-api/catalog/protocol';
import { buildCapabilityToolCatalog } from '../../../src/lib/server/public-api/catalog/tools';

const catalogTools = buildCapabilityToolCatalog(protocolVersion).tools;

describe('namespace summaries', () => {
  test('every declared namespace has a tool behind it', () => {
    const namespacesInUse = new Set(catalogTools.map((tool) => tool.namespace));
    const namespacesWithoutTools = Object.keys(capabilityNamespaceSummaries)
      .filter((namespace) => !namespacesInUse.has(namespace));

    expect(namespacesWithoutTools).toEqual([]);
  });

  test('every tool carries its namespace summary to the agent', () => {
    const toolsWithoutSummary = catalogTools
      .filter((tool) => tool.namespaceSummary === undefined)
      .map((tool) => tool.name);

    expect(toolsWithoutSummary).toEqual([]);
  });
});
