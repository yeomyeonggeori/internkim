import { afterAll, afterEach, beforeAll, expect, test } from 'bun:test';
import { readFile } from 'node:fs/promises';
import { aCompanyPlane, type ACompanyPlane } from './a-company-plane';
import {
	aTurnStartingWork,
	changingNothingTheCheckCanRead,
	everyScriptWasAskedAndNothingElse,
	expectedChangesSchemaName,
	replyingAndFinishing,
	turnRouterSchemaName,
	turnWordsOwingOnlyTheReply,
	whatTheModelWasAsked
} from './a-model-nobody-pays-for';
import { handToTheRelay, until } from './an-inbound-message';

// The agent used to learn its record tools at deploy time, from descriptors
// stamped into runtime.json. This asks the plane whether the session the relay
// opens names a catalog instead, whether the agent calls a tool it found only
// there, and whether that call reaches the record as the person who asked.

let plane: ACompanyPlane;

beforeAll(async () => {
	plane = await aCompanyPlane({ messengerPlatform: 'buzz' });
}, 180_000);

afterEach(async () => {
	await everyScriptWasAskedAndNothingElse(plane.model);
}, 90_000);

afterAll(async () => {
	await plane?.stop();
});

type ToolInventory = {
	tools: string[];
	providerByTool: Record<string, string>;
	quarantinedProviders: { providerID: string; reason: string }[];
};

async function theToolsOfTheSessionFor(email: string): Promise<ToolInventory> {
	const answer = await fetch(
		`${plane.blueclawURL}/admin/api/tools?requester=${encodeURIComponent(email)}`
	);
	expect(answer.ok).toBe(true);
	return (await answer.json()) as ToolInventory;
}

test('the agent calls a record tool the session named, and the row lands in the record', async () => {
	const [sender] = plane.people;
	const title = `평면 점검 ${Date.now()}`;

	const request = `"${title}" 업무로 남겨줘`;
	await plane.model.decideTurn(aTurnStartingWork(request, ['task_add']));
	await plane.model.answerNext(turnRouterSchemaName, turnWordsOwingOnlyTheReply);
	await plane.model.answerNext(expectedChangesSchemaName, changingNothingTheCheckCanRead);
	await plane.model.callNext('task_add', { title, type: 'task' });
	await plane.model.callNext('reply', replyingAndFinishing('업무로 남겼습니다'));

	const asked = await handToTheRelay(plane, {
		sender: { email: sender.email, name: sender.name },
		conversationID: `conversation-catalog-${plane.runIdentifier}`,
		messageID: 'message-catalog-1',
		message: request
	});
	// The door answers once the event is on disk, so whether the tool was reached
	// is read off the record rather than off this response.
	expect(asked.status, `the relay refused the turn: ${await asked.clone().text()}`).toBe(202);

	await until(
		'no task was written under that title',
		async () => (await theTasksTitled(title)).length > 0,
		120
	).catch(async (failure: Error) => {
		throw new Error(`${failure.message}\n  the model was asked: ${await whatTheModelWasAsked(plane.model)}`);
	});

	const written = await theTasksTitled(title);
	expect(written).toHaveLength(1);
	// A record tool runs as the person who asked, and task_add makes that person
	// the participant when the call names nobody else.
	expect(await theParticipantsOf(written[0].id)).toEqual([sender.memberID]);
}, 180_000);

test('a session nobody asked for keeps the tools the running host serves', async () => {
	const inventory = await theToolsOfTheSessionFor('');
	expect(inventory.providerByTool.task_add).toBe('capabilityd');
	expect(inventory.providerByTool.message_send).toBe('capabilityd');
}, 60_000);

test('the rendered runtime document stamps no capability descriptors', async () => {
	const runtimeDocument = JSON.parse(await readFile(plane.runtimeConfigurationPath, 'utf8')) as {
		capabilities: Record<string, unknown>;
	};

	expect(runtimeDocument.capabilities.unixSocketPath).toBeTruthy();
	expect(runtimeDocument.capabilities.toolDescriptors).toBeUndefined();
	expect(runtimeDocument.capabilities.protocolVersion).toBeUndefined();
	expect(runtimeDocument.capabilities.aggregateProtocolHash).toBeUndefined();
});

async function theTasksTitled(title: string): Promise<{ id: string }[]> {
	const found = await plane.admin.from('task').select('id').eq('title', title);
	if (found.error) throw new Error(found.error.message);
	return (found.data ?? []) as { id: string }[];
}

async function theParticipantsOf(taskID: string): Promise<string[]> {
	const found = await plane.admin.from('task_participant').select('member_id').eq('task_id', taskID);
	if (found.error) throw new Error(found.error.message);
	return (found.data ?? []).map((row) => (row as { member_id: string }).member_id);
}
