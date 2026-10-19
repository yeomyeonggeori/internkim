import { z } from 'zod';
import identitySchemaDocument from '../../../../internal/admind/persona-schema/identity.schema.json';
import userSchemaDocument from '../../../../internal/admind/persona-schema/user.schema.json';

type SchemaDocument = Record<string, unknown>;

function isSchemaDocument(value: unknown): value is SchemaDocument {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function withoutDefaults(schema: SchemaDocument): SchemaDocument {
	const { default: _default, properties, ...rest } = schema;
	if (!isSchemaDocument(properties)) return rest;
	const fields = Object.entries(properties).map(([name, field]) => [name, isSchemaDocument(field) ? withoutDefaults(field) : field]);
	return { ...rest, properties: Object.fromEntries(fields) };
}

function documentSchemaOf(document: SchemaDocument): z.ZodType {
	const { $schema: _dialect, $id: _identifier, title: _title, ...schema } = withoutDefaults(document);
	return z.fromJSONSchema(schema);
}

function changeSchemaOf(document: SchemaDocument): z.ZodType {
	const { properties } = withoutDefaults(document);
	if (!isSchemaDocument(properties)) throw new Error('a persona schema has no properties');
	const { schemaVersion: _schemaVersion, ...fields } = properties;
	const changeDocument: SchemaDocument = { type: 'object', additionalProperties: false, properties: fields };
	return z.fromJSONSchema(changeDocument);
}

export const personaIdentitySchema = documentSchemaOf(identitySchemaDocument);
export const personaIdentityChangeSchema = changeSchemaOf(identitySchemaDocument);
export const personaProfileSchema = documentSchemaOf(userSchemaDocument);
export const personaProfileChangeSchema = changeSchemaOf(userSchemaDocument);
