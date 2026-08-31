import { describe, expect, test } from 'bun:test';
import {
	baseTools,
	createOpenApiDocument,
	protocolVersion
} from './openapi';

const korean = createOpenApiDocument('ko');
const english = createOpenApiDocument('en');

function referencedSchemaNames(document: unknown): string[] {
	const found = [...JSON.stringify(document).matchAll(/#\/components\/schemas\/([A-Za-z]+)/g)];
	return [...new Set(found.map(([, name]) => name))];
}

describe('the API document follows the generated tool catalog', () => {
	test('every base tool has a path of its own', () => {
		const documented = Object.keys(english.paths);

		for (const tool of baseTools()) {
			expect(documented).toContain(`/tools/${tool.name}/invoke`);
		}
	});

	test('nothing is documented that the catalog does not carry', () => {
		const known = new Set(baseTools().map((tool) => tool.name));
		const invented = Object.keys(english.paths)
			.flatMap((path) => /^\/tools\/([a-z_]+)\/invoke$/.exec(path)?.[1] ?? [])
			.filter((name) => !known.has(name));

		expect(invented).toEqual([]);
	});

	test("a tool's request body carries that tool's own input schema", () => {
		const taskAdd = baseTools().find((tool) => tool.name === 'task_add');
		const documented = english.paths['/tools/task_add/invoke'] as {
			post: { requestBody: { content: Record<string, { schema: { properties: { input: object } } }> } };
		};

		const input = documented.post.requestBody.content['application/json'].schema.properties.input;
		expect(Object.keys(input)).toEqual(
			Object.keys(taskAdd?.inputSchema ?? {}).filter((key) => key !== '$schema')
		);
	});

	test('the document version is the protocol version, never a number of its own', () => {
		expect(english.info.version).toBe(protocolVersion());
		expect(korean.info.version).toBe(protocolVersion());
	});
});

describe('the document is usable as OpenAPI', () => {
	test('both languages describe the same surface', () => {
		expect(Object.keys(korean.paths).sort()).toEqual(Object.keys(english.paths).sort());
	});

	test('every referenced schema exists', () => {
		const defined = Object.keys(english.components.schemas);

		for (const name of referencedSchemaNames(english)) {
			expect(defined).toContain(name);
		}
	});

	test('a bearer token opens everything, with nothing exempt', () => {
		expect(english.security).toEqual([{ memberToken: [] }]);

		const exempt: string[] = [];
		for (const [path, methods] of Object.entries(english.paths as Record<string, Record<string, unknown>>)) {
			for (const [method, operation] of Object.entries(methods)) {
				if ((operation as { security?: unknown[] }).security?.length === 0) exempt.push(`${method} ${path}`);
			}
		}

		expect(exempt).toEqual([]);
	});

	test('every company calls one address, and a self-hosted zone replaces it', () => {
		expect(english.servers).toHaveLength(1);
		expect(english.servers[0].url).toBe('https://api.intern.kim/v1');
		expect(createOpenApiDocument('en', 'example.test').servers[0].url).toBe(
			'https://api.example.test/v1'
		);
	});
});
