import { describe, expect, test } from 'bun:test';
import { z } from 'zod';
import identitySchemaDocument from '../../../../internal/admind/persona-schema/identity.schema.json';
import userSchemaDocument from '../../../../internal/admind/persona-schema/user.schema.json';
import { personaIdentityChangeSchema, personaProfileChangeSchema, personaProfileSchema } from '../../../src/lib/persona/persona-schema';

function fieldNamesOf(schema: z.ZodType): string[] {
	const document = z.toJSONSchema(schema);
	return Object.keys(document.properties ?? {}).sort();
}

function documentFieldNamesOf(document: { properties: Record<string, unknown> }): string[] {
	return Object.keys(document.properties).filter((name) => name !== 'schemaVersion').sort();
}

describe('the persona tool schemas', () => {
	test('change exactly the fields admind’s documents carry', () => {
		expect(fieldNamesOf(personaProfileChangeSchema)).toEqual(documentFieldNamesOf(userSchemaDocument));
		expect(fieldNamesOf(personaIdentityChangeSchema)).toEqual(documentFieldNamesOf(identitySchemaDocument));
	});

	test('take one nested field alone and refuse a field the document does not name', () => {
		expect(personaProfileChangeSchema.safeParse({ morningBriefing: { enabled: false } }).success).toBe(true);
		expect(personaProfileChangeSchema.safeParse({ personID: 'somebody-else' }).success).toBe(false);
		expect(personaProfileChangeSchema.safeParse({ schemaVersion: 1 }).success).toBe(false);
	});

	test('answer the document admind keeps', () => {
		expect(personaProfileSchema.safeParse({ schemaVersion: 1, callMe: '샘플 님', morningBriefing: { enabled: true, time: '08:00' } }).success).toBe(true);
	});
});
