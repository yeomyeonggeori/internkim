import { afterAll, beforeAll, expect, test } from 'bun:test';
import { aCompanyPlane, type ACompanyPlane } from './a-company-plane';

// The agent used to learn its record tools at deploy time, from descriptors
// stamped into runtime.json. This asks the plane whether it learns them at
// session time instead, and whether a call over that path reaches the record.

let plane: ACompanyPlane;

beforeAll(async () => {
	plane = await aCompanyPlane({ messengerPlatform: 'buzz' });
}, 120_000);

afterAll(async () => {
	await plane?.stop();
});

type ToolInventory = {
	tools: string[];
	providerByTool: Record<string, string>;
	quarantinedProviders: { providerID: string; reason: string }[];
};

const recordCatalogProvider = 'mcp:internkim';
const requesterMetaKey = 'kim.intern/requester';

async function theToolsOfTheSessionFor(email: string): Promise<ToolInventory> {
	const answer = await fetch(
		`${plane.blueclawURL}/admin/api/tools?requester=${encodeURIComponent(email)}`
	);
	expect(answer.ok).toBe(true);
	return (await answer.json()) as ToolInventory;
}

// The wire blueclaw speaks: capabilityd carries MCP to admind, which runs it on
// the plane as the person named on _meta.
async function askTheCarrier(email: string, message: Record<string, unknown>): Promise<unknown> {
	const answer = await fetch('http://capability/v1/mcp', {
		unix: plane.capabilitySocketPath,
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			jsonrpc: '2.0',
			id: Date.now(),
			...message,
			params: { ...(message.params as object), _meta: { [requesterMetaKey]: email } }
		})
	});
	const body = await answer.text();
	expect(answer.status, `the carrier answered ${answer.status}: ${body}`).toBe(200);
	const answered = JSON.parse(body) as { result?: unknown; error?: unknown };
	if (answered.result === undefined) {
		throw new Error(`the carrier refused ${String(message.method)}: ${JSON.stringify(answered.error)}`);
	}
	return answered;
}

test('the agent takes its record tools from the catalog it discovered, and leaves the rest stamped', async () => {
	const [sender] = plane.people;
	const inventory = await theToolsOfTheSessionFor(sender.email);

	expect(
		inventory.providerByTool.task_add,
		`quarantined providers: ${JSON.stringify(inventory.quarantinedProviders)}`
	).toBe(recordCatalogProvider);
	expect(inventory.providerByTool.leave_request).toBe(recordCatalogProvider);
	// A tool the company machine answers is answered beside the agent, so it is
	// never taken from the plane's catalog.
	expect(inventory.providerByTool.message_send).toBe('capabilityd');
}, 60_000);

test('a session nobody asked for keeps the descriptors stamped at deploy time', async () => {
	const inventory = await theToolsOfTheSessionFor('');
	expect(inventory.providerByTool.task_add).toBe('capabilityd');
}, 60_000);

test('a record tool called over MCP writes to the record and says what it wrote', async () => {
	const [sender] = plane.people;
	const title = `평면 점검 ${Date.now()}`;

	const listed = (await askTheCarrier(sender.email, { method: 'tools/list', params: {} })) as {
		result: { tools: { name: string }[] };
	};
	expect(listed.result.tools.map((tool) => tool.name)).toContain('task_add');

	const called = (await askTheCarrier(sender.email, {
		method: 'tools/call',
		params: { name: 'task_add', arguments: { title, type: 'task' } }
	})) as { result: { isError?: boolean; structuredContent: { result: { taskID: string } } } };

	expect(called.result.isError ?? false).toBe(false);
	const taskID = called.result.structuredContent.result.taskID;
	expect(taskID).toBeTruthy();

	const written = await plane.admin.from('task').select('id, title').eq('id', taskID).single();
	expect(written.error).toBeNull();
	expect((written.data as { title: string }).title).toBe(title);
}, 60_000);
