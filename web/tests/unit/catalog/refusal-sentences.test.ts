import { expect, test } from 'bun:test';
import refusedCalls from '../../../../pkg/capabilityprotocol/refusal-sentences.json';
import { refusalOfToolInput } from '../../../src/lib/server/public-api/tool-input';

// The other half of pkg/capabilityprotocol's
// TestRefusalSentencesAreTheOnesTheCatalogPromises. A call is refused here
// against the zod the catalog is written in, and on a device against the
// descriptor that zod generated; the model reading the refusal is the same one,
// so both say it in one vocabulary.
test.each(refusedCalls.map((call) => [call.tool, call] as const))(
	'%s refuses the call in the words the capability runtime uses',
	(_tool, call) => {
		expect(refusalOfToolInput(call.tool, call.input)).toContain(call.refusal);
	}
);
