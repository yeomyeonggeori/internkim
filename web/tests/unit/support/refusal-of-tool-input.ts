import { capabilityToolInputSchema } from '../../../src/lib/server/public-api/catalog/tools';
import { parsedToolInput } from '../../../src/lib/server/public-api/tool-input';
import { InputRefused } from '../../../src/lib/server/public-api/refusal';

export function refusalOfToolInput(name: string, input: unknown): string | null {
	const schema = capabilityToolInputSchema(name);
	if (!schema) return null;
	try {
		parsedToolInput(schema, input);
		return null;
	} catch (refusal) {
		if (refusal instanceof InputRefused) return refusal.message;
		throw refusal;
	}
}
