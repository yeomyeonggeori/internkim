import { capabilityToolInputSchema } from './catalog/tools';
import { sentencesOfSchemaRefusal } from './schema-sentences';
import supersededInputFields from './catalog/superseded-input-fields.json';

type FieldSchema = { safeParse(value: unknown): { success: boolean } };

function shapeOfTool(name: string): Record<string, FieldSchema> | null {
	const schema = capabilityToolInputSchema(name);
	return (schema as unknown as { shape?: Record<string, FieldSchema> })?.shape ?? null;
}

// The caller this API was built for is a model, and a model has three ways of
// saying a field has no value: it leaves the field out, it writes null, or it
// writes "". Only the first was taken. null is not undefined to a schema, and
// "" is not one of XS|S|M to an enum, so two of the three were refused for
// their spelling — 52 of 105 checks over the catalog.
//
// Widening the schemas is not the fix: a nullable type is the shape the
// tool-calling backends disagree about, and the catalog is written to the least
// common denominator on purpose. The three are made one here instead. A field
// the schema lets be left out, written with nothing in it, is left out — which
// is what the caller said, in the spelling the schema already understands.
//
// A field that must be given is untouched: dropping a blank there would turn
// "you sent nothing" into "you forgot this", and the caller needs the first.
function fieldsTheToolLetsBeLeftOut(name: string): Map<string, FieldSchema> {
	const shape = shapeOfTool(name);
	if (!shape) return new Map();
	return new Map(Object.entries(shape).filter(([, field]) => field.safeParse(undefined).success));
}

// A device asks with the catalog its own release shipped with, so a fleet that
// has not taken the latest OTA still names a field this one has renamed.
// Refusing that deadlocks the caller: its copy of the catalog refuses the name
// that replaced it, so no reply can tell it what to send instead. The call is
// read under the name the catalog now uses. Nothing publishes the old name, so
// no model learns it here.
//
// A retired hint was one person and the field that replaced it is a list, and
// scope only ever answered whose records to read when that hint was empty, so a
// call that carried both is read the way the catalog it was written against
// defined it.
function callUnderTodaysFieldNames(name: string, input: Record<string, unknown>): Record<string, unknown> {
	const shape = shapeOfTool(name);
	if (!shape) return input;
	const asked = { ...input };
	for (const { was, isNow } of supersededInputFields) {
		if (!(was in asked) || was in shape || !(isNow in shape)) continue;
		const named = asked[was];
		delete asked[was];
		if (typeof named !== 'string' || named.trim() === '') continue;
		asked[isNow] = [named];
		delete asked.scope;
	}
	return asked;
}

// null is not a value any of these schemas take, so it is always the caller
// saying nothing. An empty string is one only where the field refuses it: "" is
// not one of XS|S|M, so it means the size was left out. A field that takes an
// empty string takes it as a value, and the catalog says what it means on the
// fields that do — an empty organizationHint takes the work off the
// organization rather than leaving it where it was.
function saysNothing(value: unknown, field: FieldSchema): boolean {
	if (value === null) return true;
	if (typeof value !== 'string' || value.trim() !== '') return false;
	return !field.safeParse(value).success;
}

export function toolInputRecovered(name: string, input: unknown): Record<string, unknown> {
	if (input === null || input === undefined) return {};
	if (typeof input !== 'object' || Array.isArray(input)) return input as Record<string, unknown>;
	const mayBeLeftOut = fieldsTheToolLetsBeLeftOut(name);
	const asked = callUnderTodaysFieldNames(name, input as Record<string, unknown>);
	return Object.fromEntries(
		Object.entries(asked).filter(([field, value]) => {
			const schema = mayBeLeftOut.get(field);
			return !schema || !saysNothing(value, schema);
		})
	);
}

export function refusalOfToolInput(name: string, input: unknown): string | null {
	const schema = capabilityToolInputSchema(name);
	if (!schema) return null;
	const parsed = schema.safeParse(input);
	if (parsed.success) return null;
	return sentencesOfSchemaRefusal(parsed.error.issues, input, 'input');
}
