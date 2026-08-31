import { beforeEach, describe, expect, test } from 'bun:test';
import { createOpenApiDocument } from '../../../web/src/lib/server/openapi';
import worker, { type CompanyAnswer, type CompanyCall, type WorkerEnvironment } from './index';

type Operation = { path: string; method: string };

type PathsOfDocument = Record<string, Record<string, unknown> | undefined>;

function operationsOf(): Operation[] {
	const paths = (createOpenApiDocument('en') as { paths: PathsOfDocument }).paths;
	const operations: Operation[] = [];
	for (const [path, methods] of Object.entries(paths)) {
		for (const method of Object.keys(methods ?? {})) operations.push({ path, method });
	}
	return operations;
}

Object.assign(globalThis, {
	fetch: async (url: string, options: { method?: string } = {}) => {
		if (String(url).includes('/storage/v1/object/')) return { ok: true, status: 200, json: async () => null };
		if ((options.method ?? 'GET') !== 'GET') return { ok: true, status: 200, json: async () => [{ name: 'made' }] };
		if (String(url).includes('external_id=')) {
			return {
				ok: true,
				status: 200,
				json: async () => [
					{ name: 'holder', permission: 'delete', member: { id: 'm1', email: 'someone@example.com', company_id: 'c1' } }
				]
			};
		}
		return { ok: true, status: 200, json: async () => [] };
	}
});

const environment: WorkerEnvironment = {
	COMPANY_CALLS: {
		async callCompany(_companyID: string, call: CompanyCall): Promise<CompanyAnswer> {
			return { requestID: call.requestID, status: 200, body: { ok: true } };
		}
	},
	SUPABASE_URL: 'https://plane.supabase.co',
	SUPABASE_SECRET_KEY: 'service-role'
};

// A documented path carries {name} where a real call carries a tool. The worker
// only has to recognise the shape, so any name it can resolve will do.
function addressOf(path: string): string {
	return path.replace('{name}', 'task_list');
}

function callDocumented(operation: Operation): Promise<Response> {
	const method = operation.method.toUpperCase();
	const carriesBody = method === 'POST' || method === 'PUT' || method === 'PATCH';
	const address = `https://api.intern.kim/v1${addressOf(operation.path)}${method === 'DELETE' ? '?name=other' : ''}`;
	return worker.fetch(
		new Request(address, {
			method,
			headers: { Authorization: 'Bearer ik_holder', 'Content-Type': 'application/json' },
			...(carriesBody ? { body: '{}' } : {})
		}),
		environment
	);
}

describe('every documented endpoint answers', () => {
	let operations: Operation[] = [];

	beforeEach(() => {
		operations = operationsOf();
	});

	test('the document is not empty', () => {
		expect(operations.length).toBeGreaterThan(30);
	});

	test('no documented path is unreachable', async () => {
		const unreachable: string[] = [];
		for (const operation of operations) {
			const response = await callDocumented(operation);
			if (response.status === 404 || response.status === 405) {
				unreachable.push(`${operation.method.toUpperCase()} ${operation.path} -> ${response.status}`);
			}
		}
		expect(unreachable).toEqual([]);
	});

	test('no documented path answers as if it carried no token', async () => {
		const refused: string[] = [];
		for (const operation of operations) {
			const response = await callDocumented(operation);
			if (response.status === 401) refused.push(`${operation.method.toUpperCase()} ${operation.path}`);
		}
		expect(refused).toEqual([]);
	});

	test('the paths the worker answers are all documented', () => {
		const documented = new Set(operationsOf().map((operation) => `${operation.method} ${operation.path}`));
		const served = [
			'get /tools',
			'get /tools/{name}',
			'post /tools/{name}/invoke',
			'get /tokens',
			'post /token',
			'delete /token',
			'post /files',
			'post /agent/messages',
			'get /agent/replies'
		];
		expect(served.filter((operation) => !documented.has(operation))).toEqual([]);
	});
});
