import { expect } from 'bun:test';
import { capabilityToolResultSchema } from '../../src/lib/server/public-api/catalog/tools';

type ToolAnswer = { status: number; body: unknown };

// The plane takes the catalog when it deploys and a device takes it when an OTA
// release reaches it, so an answer this suite accepts and the published contract
// refuses is every task tool on the fleet refusing its own answer until the next
// release (#1486). Holding it here is what makes an answer readable where it is
// read, rather than only where it is produced.
//
// Every suite that invokes a tool passes its answers through this, so a tool
// nobody thought to check is checked anyway. A per-call assertion is always the
// one missing on the call that breaks.
export function heldToTheContract<Answer extends ToolAnswer>(name: string, answer: Answer): Answer {
	if (answer.status !== 200) return answer;
	const schema = capabilityToolResultSchema(name);
	if (!schema) throw new Error(`${name} publishes no result contract to hold its answer to`);
	const parsed = schema.safeParse((answer.body as { result: unknown }).result);
	const refused = parsed.success
		? []
		: parsed.error.issues.map((issue) => `${['result', ...issue.path.map(String)].join('.')}: ${issue.message}`);
	expect({ tool: name, refused }).toEqual({ tool: name, refused: [] });
	return answer;
}
