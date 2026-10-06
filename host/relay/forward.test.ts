import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import {
	admindAPIURL,
	answerBodyOf,
	apiRequestCapability,
	askAdmind,
	forwardToAdmind,
	forwardToAdmindAPI,
	forwardToChatd,
	isPersonCapability,
	isRegistrationCapability,
	isWorkspaceCapability,
	mailOperationOf,
	publicAPIRequestOf,
	publicAPIFileOf,
	serveCallForMember,
	servePublicAPIFile,
	servePublicAPIRequest,
	serveTelling,
	tellCallOf,
	workspaceCallOf,
	workspaceCapabilityPaths,
	workspaceDownloadURL,
	workspaceFileURL,
	type AdmindCall,
	type ConnectedAccount,
	type PublicAPIRequest
} from './forward';
import { leafNameOf, type KeptFileReference } from './file-transfer';
import { TransferFailed } from './transfer-store';
import { MessengerAnswered } from './person-picture';
import { rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

function dispatchThatKnows(externalIDs: Record<string, string>) {
	const asked: { capability: string; body: Record<string, unknown> }[] = [];
	const connected: { memberID: string; account: ConnectedAccount }[] = [];
	const admindCalls: AdmindCall[] = [];
	const arrivals: { conversationID: string; messageID: string }[] = [];
	const transfers: { operation: string; memberID: string; requester?: string; actor?: unknown; body: unknown }[] = [];
	return {
		asked,
		connected,
		admindCalls,
		arrivals,
		transfers,
		dispatch: {
			connectMessengerAccount: async (memberID: string, account: ConnectedAccount) => {
				connected.push({ memberID, account });
			},
			serveAsset: async (capability: string) => ({ served: capability }),
			serveMessenger: async (capability: string) => ({ status: 200, body: { served: capability } }),
			askMaild: async (operation: string, body: Record<string, unknown>) => {
				asked.push({ capability: `mail.${operation}`, body });
				return { status: 200, body: { mailboxes: [] } };
			},
			mailAccountOf: async (memberID: string) =>
				memberID === 'member-1' ? { imapHost: 'imap.example.test' } : null,
			askChatd: async (capability: string, body: Record<string, unknown>) => {
				asked.push({ capability, body });
				if (capability === 'person.identity') {
					const actor = body.actor as { secret?: string } | undefined;
					const externalID = actor?.secret === 'known' ? 'U-known' : 'U-stranger';
					return { status: 200, body: { externalID } };
				}
				if (capability === 'person.credential.issue') {
					return {
						status: 200,
						body: {
							credential: { kind: 'mattermost-token', secret: 'a-durable-token' },
							identity: { externalID: 'U-new', name: '이샘플' }
						}
					};
				}
				return { status: 200, body: { conversations: [] } };
			},
			askAdmindAPI: async (request: PublicAPIRequest) => {
				asked.push({ capability: apiRequestCapability, body: { ...request } });
				return { status: 200, body: { served: request.path } };
			},
			tellAdmindTheDirectoryChanged: async () => ({ status: 202, body: null }),
			emailOfMember: async (memberID: string) =>
				memberID === 'member-1' ? 'sample@example.test' : null,
			messengerCredentialOf: async () => ({ kind: 'buzz-secret', secret: 'a-held-secret' }),
			messageArrived: (conversationID: string, messageID: string) => {
				arrivals.push({ conversationID, messageID });
			},

			memberOfExternalID: async (externalID: string) => externalIDs[externalID] ?? null,
			transfer: {
				prepareMedia: async (memberID: string, actor: unknown, body: Record<string, unknown>) => {
					transfers.push({ operation: 'prepareMedia', memberID, actor, body });
					return { status: 200, body: { transfer: { state: 'pending' } } };
				},
				takeUploadIntoMessenger: async (memberID: string, actor: unknown, requester: string, body: Record<string, unknown>) => {
					transfers.push({ operation: 'takeUploadIntoMessenger', memberID, actor, requester, body });
					return { status: 200, body: { transfer: { state: 'pending' } } };
				},
				prepareWorkspaceFile: async (memberID: string, requester: string, body: Record<string, unknown>) => {
					transfers.push({ operation: 'prepareWorkspaceFile', memberID, requester, body });
					return { status: 200, body: { transfer: { state: 'pending' } } };
				},
				takeUploadIntoWorkspace: async (memberID: string, requester: string, body: Record<string, unknown>) => {
					transfers.push({ operation: 'takeUploadIntoWorkspace', memberID, requester, body });
					return { status: 200, body: { transfer: { state: 'pending' } } };
				},
				writeKeptFileIntoWorkspace: async (kept: KeptFileReference, path: string) => {
					transfers.push({ operation: 'writeKeptFileIntoWorkspace', memberID: '', requester: kept.requester, body: { kept, path } });
					return { path, sizeBytes: 'kept bytes'.length };
				}
			},
			keptPersonPicture: async ({ externalID, avatarURL }: { externalID: string; avatarURL: string }) => {
				asked.push({ capability: 'person.picture', body: { externalID, avatarURL } });
				return externalID === 'npub-bare' ? '' : keptAddress(`person-picture/${externalID}`);
			},
			askAdmindAsRequester: async (call: AdmindCall) => {
				admindCalls.push(call);
				return { status: 200, body: { served: call.url } };
			}
		}
	};
}

function keptAddress(digest: string): string {
	return `https://company.supabase.co/storage/v1/object/asset/company-1/shared/attachment/${digest}`;
}

describe('persona identity forwarding', () => {
	test('allows identity reads and admin updates while removing soul writes', () => {
		const read = workspaceCallOf('person.persona.identity', {}, 'sample@example.test');
		expect(read).toEqual({ method: 'GET', url: 'http://internkim/persona/api/identity', requester: 'sample@example.test' });
		const write = workspaceCallOf('person.persona.identity.update', { identity: { names: ['이샘플'] }, actor: { role: 'admin' } }, 'sample@example.test');
		expect(write).toMatchObject({ method: 'POST', url: 'http://internkim/persona/api/identity', requester: 'sample@example.test' });
		expect(write?.body).toContain('이샘플');
		expect(workspaceCallOf('person.persona.soul.update', {}, 'sample@example.test')).toBeNull();
	});
});

describe('isPersonCapability', () => {
	test('person capabilities need an actor, assets do not', () => {
		expect(isPersonCapability('person.conversations.list')).toBe(true);
		expect(isPersonCapability('person.message.send')).toBe(true);
		expect(isPersonCapability('person.emoji.list')).toBe(true);
		expect(isPersonCapability('person.picture')).toBe(true);
		expect(isPersonCapability('asset.link')).toBe(false);
	});
});

describe('serveCallForMember', () => {
	const callFromTheBrowser = {
		callID: 'c1',
		capability: 'person.conversations.list',
		body: {}
	};

	test('an asset the messenger does not own is served without a credential', async () => {
		const { dispatch } = dispatchThatKnows({ 'U-known': 'member-1' });

		const served = await serveCallForMember(dispatch, {
			callID: 'c1',
			capability: 'asset.link',
			body: {}
		}, 'member-1');

		expect(served).toEqual({
			status: 200,
			body: { served: 'asset.link' },
			replyTo: 'member-1'
		});
	});

	test('a workspace call is asked of the workspace as the person who asked', async () => {
		const { admindCalls, dispatch } = dispatchThatKnows({ 'U-known': 'member-1' });

		const served = await serveCallForMember(dispatch, {
			callID: 'c1',
			capability: 'person.memory.facts',
			body: { limit: 50 }
		}, 'member-1');

		expect(served.status).toBe(200);
		expect(served.replyTo).toBe('member-1');
		expect(admindCalls).toEqual([
			{
				method: 'GET',
				url: 'http://internkim/memory/api/facts?limit=50',
				requester: 'sample@example.test'
			}
		]);
	});

	test('a workspace capability the app does not have is refused rather than asked for', async () => {
		const { admindCalls, dispatch } = dispatchThatKnows({});

		const served = await serveCallForMember(dispatch, {
			callID: 'c1',
			capability: 'person.files.invented',
			body: {}
		}, 'member-1');

		expect(served.status).toBe(404);
		expect(admindCalls).toEqual([]);
	});

	test('a workspace call for a member the record has no address for is refused', async () => {
		const { dispatch } = dispatchThatKnows({});

		const served = await serveCallForMember(dispatch, {
			callID: 'c1',
			capability: 'person.memory.facts',
			body: {}
		}, 'member-2');

		expect(served.status).toBe(409);
	});

});

describe('a message the app sent', () => {
	test('is shown to the browsers once the messenger took it', async () => {
		const { arrivals, dispatch } = dispatchThatKnows({});

		const served = await serveCallForMember(
			{
				...dispatch,
				askChatd: async (capability: string) =>
					capability === 'person.message.send' ? { status: 200, body: { id: 'event-1' } } : { status: 200, body: {} }
			},
			{ callID: 'c1', capability: 'person.message.send', body: { conversationID: 'channel-1', body: '지금 갈게요' } },
			'member-1'
		);

		expect(served.status).toBe(200);
		expect(arrivals).toEqual([{ conversationID: 'channel-1', messageID: 'event-1' }]);
	});
});

describe('a file moving between a browser and this machine', () => {
	test('is prepared for reading as the member who asked, with their own messenger credential', async () => {
		const { dispatch, transfers } = dispatchThatKnows({});

		const served = await serveCallForMember(
			dispatch,
			{ capability: 'person.media.prepare', body: { transferID: 'transfer-1', mediaURL: 'https://relay.test/media/a.png' } },
			'member-1'
		);

		expect(served).toEqual({ status: 200, body: { transfer: { state: 'pending' } }, replyTo: 'member-1' });
		expect(transfers).toEqual([
			{
				operation: 'prepareMedia',
				memberID: 'member-1',
				actor: { kind: 'buzz-secret', secret: 'a-held-secret' },
				body: { transferID: 'transfer-1', mediaURL: 'https://relay.test/media/a.png' }
			}
		]);
	});

	test('an upload for the messenger is taken as the member, under their own address', async () => {
		const { dispatch, transfers } = dispatchThatKnows({});

		await serveCallForMember(dispatch, { capability: 'person.media.upload', body: { transferID: 'transfer-2' } }, 'member-1');

		expect(transfers[0]).toMatchObject({ operation: 'takeUploadIntoMessenger', memberID: 'member-1', requester: 'sample@example.test' });
	});

	test('a workspace file is read and written as the member, never through a workspace route', async () => {
		const { dispatch, transfers, admindCalls } = dispatchThatKnows({});

		await serveCallForMember(dispatch, { capability: 'person.files.download', body: { transferID: 'transfer-3', path: '/workspace/a' } }, 'member-1');
		await serveCallForMember(dispatch, { capability: 'person.files.upload', body: { transferID: 'transfer-4' } }, 'member-1');

		expect(transfers.map((one) => [one.operation, one.requester])).toEqual([
			['prepareWorkspaceFile', 'sample@example.test'],
			['takeUploadIntoWorkspace', 'sample@example.test']
		]);
		expect(admindCalls).toEqual([]);
	});

	test('a member the record has no address for moves nothing', async () => {
		const { dispatch, transfers } = dispatchThatKnows({});

		const served = await serveCallForMember(dispatch, { capability: 'person.files.download', body: {} }, 'member-2');

		expect(served.status).toBe(409);
		expect(transfers).toEqual([]);
	});

	test('a member with no messenger account cannot read the messenger', async () => {
		const { dispatch, transfers } = dispatchThatKnows({});

		const served = await serveCallForMember(
			{ ...dispatch, messengerCredentialOf: async () => null },
			{ capability: 'person.media.prepare', body: { transferID: 'transfer-5' } },
			'member-1'
		);

		expect(served.status).toBe(409);
		expect(transfers).toEqual([]);
	});
});

describe('the picture of a person', () => {
	const read = (body: Record<string, unknown>) => ({ callID: 'c1', capability: 'person.picture', body });

	test('is answered with an address in the company bucket the reader signs for, never with bytes', async () => {
		const { asked, dispatch } = dispatchThatKnows({});

		const served = await serveCallForMember(
			dispatch,
			read({ externalID: 'npub-drawn', avatarURL: 'https://relay.example.com/media/abc.png' }),
			'member-1'
		);

		expect(served).toEqual({
			status: 200,
			body: { picture: { address: keptAddress('person-picture/npub-drawn') } },
			replyTo: 'member-1'
		});
		expect(asked).toEqual([
			{ capability: 'person.picture', body: { externalID: 'npub-drawn', avatarURL: 'https://relay.example.com/media/abc.png' } }
		]);
	});

	test('someone who set none has none', async () => {
		const { dispatch } = dispatchThatKnows({});
		const served = await serveCallForMember(dispatch, read({ externalID: 'npub-bare' }), 'member-1');
		expect(served.body).toEqual({ picture: null });
	});

	test('a messenger that could not hand it over is answered as the failure it is', async () => {
		const { dispatch } = dispatchThatKnows({});
		const served = await serveCallForMember(
			{
				...dispatch,
				keptPersonPicture: async () => {
					throw new MessengerAnswered(502, 'person.picture answered 502');
				}
			},
			read({ externalID: 'npub-drawn' }),
			'member-1'
		);
		expect(served.status).toBe(502);
		expect(served.body).toEqual({ picture: null, error: 'person.picture answered 502' });
	});

	test('is asked after by account, or not at all', async () => {
		const { dispatch } = dispatchThatKnows({});
		expect((await serveCallForMember(dispatch, read({}), 'member-1')).status).toBe(400);
	});
});

describe('mailOperationOf', () => {
	test('a mail capability names an operation maild knows', () => {
		expect(mailOperationOf('person.mail.messages')).toBe('messages');
		expect(mailOperationOf('person.mail.send')).toBe('send');
	});

	test('anything else is not mail', () => {
		expect(mailOperationOf('person.message.send')).toBeNull();
		expect(mailOperationOf('person.mail.')).toBeNull();
		expect(mailOperationOf('asset.link')).toBeNull();
	});

	test('an operation cannot be a path', () => {
		expect(mailOperationOf('person.mail.../secrets')).toBeNull();
		expect(mailOperationOf('person.mail.send/../test')).toBeNull();
	});
});

describe('serveCallForMember for mail', () => {
	test('the mail account is fetched for the member the gateway named', async () => {
		const { asked, dispatch } = dispatchThatKnows({ 'U-known': 'member-1' });

		const served = await serveCallForMember(dispatch, {
			callID: 'c1',
			capability: 'person.mail.mailboxes',
			body: {}
		}, 'member-1');

		expect(served.status).toBe(200);
		expect(served.replyTo).toBe('member-1');
		expect(asked.map((entry) => entry.capability)).toEqual(['mail.mailboxes']);
		expect(asked[0]?.body.account).toEqual({ imapHost: 'imap.example.test' });
	});

	test('maild is told whose account it carries, so a password sealed to that member opens and no other', async () => {
		const { asked, dispatch } = dispatchThatKnows({});

		await serveCallForMember(dispatch, {
			callID: 'c1',
			capability: 'person.mail.mailboxes',
			body: { memberID: 'member-2' }
		}, 'member-1');

		expect(asked[0]?.body.memberID).toBe('member-1');
	});

	test('the browser never carries the mail password, so one it offers is ignored', async () => {
		const { asked, dispatch } = dispatchThatKnows({});

		await serveCallForMember(dispatch, {
			callID: 'c1',
			capability: 'person.mail.mailboxes',
			body: { account: { imapHost: 'imap.attacker.test', imapPassword: 'stolen' } }
		}, 'member-1');

		expect(asked[0]?.body.account).toEqual({ imapHost: 'imap.example.test' });
	});

	test('a member who connected no mail account is told so, and maild is not asked', async () => {
		const { asked, dispatch } = dispatchThatKnows({});

		const served = await serveCallForMember(dispatch, {
			callID: 'c1',
			capability: 'person.mail.mailboxes',
			body: {}
		}, 'member-2');

		expect(served.status).toBe(409);
		expect(asked).toEqual([]);
	});
});

describe('answerBodyOf', () => {
	test('carries the reason a service wrote as plain text', async () => {
		const refused = new Response('workspace access required', { status: 403 });

		expect(await answerBodyOf(refused)).toEqual({ error: 'workspace access required' });
	});

	test('reads a JSON answer as itself', async () => {
		const answered = new Response(JSON.stringify({ roots: [] }), {
			status: 200,
			headers: { 'content-type': 'application/json' }
		});

		expect(await answerBodyOf(answered)).toEqual({ roots: [] });
	});

	test('leaves a body that says nothing as nothing', async () => {
		expect(await answerBodyOf(new Response('', { status: 204 }))).toBe(null);
	});

	test('keeps plain text that came back with a success', async () => {
		expect(await answerBodyOf(new Response('done', { status: 200 }))).toBe('done');
	});
});

describe('forwardToChatd', () => {
	const realFetch = globalThis.fetch;

	afterEach(() => {
		globalThis.fetch = realFetch;
	});

	async function bodySentFor(body: Record<string, unknown>): Promise<Record<string, unknown>> {
		let sent: Record<string, unknown> = {};
		globalThis.fetch = (async (_input: RequestInfo | URL, init?: RequestInit) => {
			sent = JSON.parse(String(init?.body));
			return Response.json({ image: null });
		}) as typeof fetch;
		await forwardToChatd('http://chatd.test', 'mattermost', 'person.picture', body, 674_000);
		return sent;
	}

	test('tells chatd what this transport can carry, so a drawing is never too large to send', async () => {
		expect(await bodySentFor({ externalID: 'U1' })).toEqual({
			externalID: 'U1',
			largestBytes: 674_000
		});
	});

	test('a caller cannot raise the limit past what this transport carries', async () => {
		expect((await bodySentFor({ largestBytes: 99_000_000 })).largestBytes).toBe(674_000);
	});

	test('names the address it could not reach, because fetch never says which one it tried', async () => {
		globalThis.fetch = (async () => {
			throw new Error('Unable to connect. Is the computer able to access the url?');
		}) as unknown as typeof fetch;

		const refusal = await forwardToChatd(
			'http://127.0.0.1:18090',
			'buzz',
			'person.messages.list',
			{},
			674_000
		).catch((failure: unknown) => failure);

		expect(refusal).toBeInstanceOf(Error);
		expect((refusal as Error).message).toContain('http://127.0.0.1:18090');
		expect((refusal as Error).message).toContain('Unable to connect');
	});
});


describe('a member connects their own messenger account', () => {
	test('registration is answered on the channel it arrived on, with no credential at all', async () => {
		const { connected, dispatch } = dispatchThatKnows({});

		const served = await serveCallForMember(
			dispatch,
			{ callID: 'c1', capability: 'person.credential.requirement' },
			'member-1'
		);

		expect(served.status).toBe(200);
		expect(served.replyTo).toBe('member-1');
		expect(connected).toEqual([]);
	});

	test('an issued credential is kept for the member the channel belongs to', async () => {
		const { connected, dispatch } = dispatchThatKnows({});

		await serveCallForMember(
			dispatch,
			{ callID: 'c1', capability: 'person.credential.issue', body: { answers: { password: 'x' } } },
			'member-1'
		);

		expect(connected).toEqual([
			{
				memberID: 'member-1',
				account: {
					kind: 'mattermost-token',
					externalID: 'U-new',
					name: '이샘플',
					secret: 'a-durable-token'
				}
			}
		]);
	});

	test('the answer never carries the secret back to the browser', async () => {
		const { dispatch } = dispatchThatKnows({});

		const served = await serveCallForMember(
			dispatch,
			{ callID: 'c1', capability: 'person.credential.issue', body: { answers: { password: 'x' } } },
			'member-1'
		);

		expect(served.body).toEqual({ externalID: 'U-new', name: '이샘플' });
		expect(JSON.stringify(served)).not.toContain('a-durable-token');
	});

	test('only the credential pair authenticates by channel', () => {
		expect(isRegistrationCapability('person.credential.requirement')).toBe(true);
		expect(isRegistrationCapability('person.credential.issue')).toBe(true);
		expect(isRegistrationCapability('person.message.send')).toBe(false);
		expect(isRegistrationCapability('person.identity')).toBe(false);
	});
});

test('a buzz claim reaches the workspace rather than the messenger', () => {
	expect(isWorkspaceCapability('person.buzz.claim')).toBe(true);
	expect(isWorkspaceCapability('person.buzz.relay')).toBe(true);
});

test('a message is still the messenger, not the workspace', () => {
	expect(isWorkspaceCapability('person.message.send')).toBe(false);
});

describe('every capability the workspace family names', () => {
	const capabilities = Object.keys(workspaceCapabilityPaths);

	test('is asked for on the socket admind hears a requester on, never at a loopback port', () => {
		expect(capabilities.length).toBeGreaterThan(0);
		for (const capability of capabilities) {
			const call = workspaceCallOf(capability, {}, 'sample@example.test');
			expect(`${capability} ${call?.url}`).toBe(
				`${capability} http://internkim${workspaceCapabilityPaths[capability]}`
			);
			expect(call?.requester).toBe('sample@example.test');
		}
	});

	test('is one the relay routes to the workspace rather than the messenger', () => {
		for (const capability of capabilities) {
			expect(isWorkspaceCapability(capability)).toBe(true);
		}
	});

	test('asks for one call\'s exchange, a turn\'s input and the inbound messages by what the caller named', () => {
		expect(workspaceCallOf('person.runs.llm_call', { id: 'call-1' }, 'sample@example.test')?.url).toBe('http://internkim/runs/api/llm-call?id=call-1');
		expect(workspaceCallOf('person.runs.inbound', { limit: 50 }, 'sample@example.test')?.url).toBe('http://internkim/runs/api/inbound?limit=50');
		expect(workspaceCallOf('person.runs.turn_input', { id: 'event-1' }, 'sample@example.test')?.url).toBe('http://internkim/runs/api/turn-input?id=event-1');
	});

	test('carries what the caller asked for as a query, and never the actor', () => {
		const call = workspaceCallOf(
			'person.runs.detail',
			{ taskRunID: 'run-1', actor: { kind: 'buzz' } },
			'sample@example.test'
		);

		expect(call?.url).toBe('http://internkim/runs/api/detail?taskRunID=run-1');
	});

	test('a quick task is posted as a json body, and never the actor', () => {
		expect(isWorkspaceCapability('person.task.quick_task')).toBe(true);
		const call = workspaceCallOf(
			'person.task.quick_task',
			{ prompt: '내일까지 보고서', weekCode: '26W35', allowDuplicate: false, actor: { kind: 'buzz' } },
			'sample@example.test'
		);

		expect(call?.method).toBe('POST');
		expect(call?.url).toBe('http://internkim/task/api/tasks/quick');
		expect(call?.contentType).toBe('application/json');
		expect(JSON.parse(String(call?.body))).toEqual({
			prompt: '내일까지 보고서',
			weekCode: '26W35',
			allowDuplicate: false
		});
		expect(call?.requester).toBe('sample@example.test');
	});

	test('an approval decision is posted to the workspace as the person who signed in', () => {
		expect(isWorkspaceCapability('person.runs.approve')).toBe(true);
		const call = workspaceCallOf(
			'person.runs.approve',
			{ taskRunID: 'run-1', decision: 'approve', actor: { kind: 'buzz' } },
			'sample@example.test'
		);

		expect(call?.method).toBe('POST');
		expect(call?.url).toBe('http://internkim/runs/api/approve');
		expect(call?.contentType).toBe('application/json');
		expect(JSON.parse(String(call?.body))).toEqual({ taskRunID: 'run-1', decision: 'approve' });
		expect(call?.requester).toBe('sample@example.test');
	});

	test('a retry request is posted with the actor removed', () => {
		const call = workspaceCallOf(
			'person.runs.retry',
			{ taskRunID: 'run-1', actor: { kind: 'forged' }, viewerEmail: 'forged@example.test', viewerIsAdmin: false },
			'sample@example.test'
		);

		expect(call?.method).toBe('POST');
		expect(call?.url).toBe('http://internkim/runs/api/retry');
		expect(call?.contentType).toBe('application/json');
		expect(JSON.parse(String(call?.body))).toEqual({
			taskRunID: 'run-1',
			viewerEmail: 'forged@example.test',
			viewerIsAdmin: false
		});
		expect(call?.requester).toBe('sample@example.test');
	});

	test('the skill inventory is a workspace read, not a message to the agent', () => {
		expect(isWorkspaceCapability('person.skills.list')).toBe(true);
		const call = workspaceCallOf('person.skills.list', { actor: { kind: 'buzz' } }, 'sample@example.test');

		expect(call?.method).toBe('GET');
		expect(call?.url).toBe('http://internkim/skills/api');
		expect(call?.requester).toBe('sample@example.test');
	});

	test('every memory change is posted to the workspace as the person who signed in', () => {
		const changes: [string, string, Record<string, unknown>][] = [
			['person.memory.schedule_cancel', '/memory/api/schedules/cancel', { taskScheduleID: 'schedule-1' }],
			['person.memory.schedule_delete', '/memory/api/schedules/delete', { taskScheduleID: 'schedule-1' }],
			['person.memory.schedule_update', '/memory/api/schedules/update', { taskScheduleID: 'schedule-1', name: '주간 보고' }]
		];

		for (const [capability, path, body] of changes) {
			expect(isWorkspaceCapability(capability)).toBe(true);
			const call = workspaceCallOf(capability, { ...body, actor: { kind: 'buzz' } }, 'sample@example.test');

			expect(`${capability} ${call?.method} ${call?.url}`).toBe(`${capability} POST http://internkim${path}`);
			expect(call?.contentType).toBe('application/json');
			expect(JSON.parse(String(call?.body))).toEqual(body);
			expect(call?.requester).toBe('sample@example.test');
		}
	});
});

type ArrivedRequest = {
	method: string;
	path: string;
	search: string;
	headers: Record<string, string>;
	body: string;
};

function admindListeningOn(socketPath: string, arrived: ArrivedRequest[]) {
	return Bun.serve({
		unix: socketPath,
		fetch: async (request) => {
			const asked = new URL(request.url);
			const headers: Record<string, string> = {};
			request.headers.forEach((value, name) => {
				headers[name] = value;
			});
			arrived.push({
				method: request.method,
				path: asked.pathname,
				search: asked.search,
				headers,
				body: await request.text()
			});
			return Response.json({ invoked: true });
		}
	});
}

describe('a public API call the plane resolved', () => {
	const arrived: ArrivedRequest[] = [];
	let socketPath = '';
	let admind: ReturnType<typeof admindListeningOn> | null = null;

	function dispatchReachingTheSocket() {
		const { dispatch } = dispatchThatKnows({});
		return {
			...dispatch,
			askAdmindAPI: (request: PublicAPIRequest) => forwardToAdmindAPI(socketPath, request)
		};
	}

	const invoke = {
		method: 'POST',
		path: '/tools/message_send/invoke',
		query: '',
		permission: 'write',
		requester: 'sample@example.test',
		payload: { conversationID: 'channel-1', body: 'hello' }
	};

	beforeEach(() => {
		arrived.length = 0;
		socketPath = join(tmpdir(), `relay-api-${crypto.randomUUID().slice(0, 8)}.sock`);
		admind = admindListeningOn(socketPath, arrived);
	});

	afterEach(() => {
		admind?.stop(true);
		admind = null;
		rmSync(socketPath, { force: true });
	});

	test('is handed to admind over the socket under the name the plane resolved', async () => {
		const served = await servePublicAPIRequest(dispatchReachingTheSocket(), invoke);

		expect(served.status).toBe(200);
		expect(served.body).toEqual({ invoked: true });
		expect(arrived).toHaveLength(1);
		expect(arrived[0]?.method).toBe('POST');
		expect(arrived[0]?.path).toBe('/api/v1/tools/message_send/invoke');
		expect(arrived[0]?.headers['x-internkim-requester-email']).toBe('sample@example.test');
		expect(arrived[0]?.headers['x-internkim-requester-permission']).toBe('write');
		expect(JSON.parse(arrived[0]?.body ?? 'null')).toEqual(invoke.payload);
	});

	test('carries no header the caller wrote into the body', async () => {
		const smuggled = {
			...invoke,
			headers: { 'X-INTERNKIM-REQUESTER-EMAIL': 'chief@example.test' },
			'X-INTERNKIM-REQUESTER-EMAIL': 'chief@example.test',
			'x-internkim-requester-permission': 'admin',
			Authorization: 'Bearer somebody-elses-key'
		};

		const served = await servePublicAPIRequest(dispatchReachingTheSocket(), smuggled);

		expect(served.status).toBe(200);
		const headers = arrived[0]?.headers ?? {};
		expect(headers['x-internkim-requester-email']).toBe('sample@example.test');
		expect(headers['x-internkim-requester-permission']).toBe('write');
		expect(headers.authorization).toBeUndefined();
		expect(Object.keys(headers).filter((name) => name.startsWith('x-internkim'))).toHaveLength(2);
	});

	test('a requester carrying a second header line is refused rather than sent', async () => {
		const served = await servePublicAPIRequest(dispatchReachingTheSocket(), {
			...invoke,
			requester: 'sample@example.test\r\nX-Internkim-Requester-Permission: admin'
		});

		expect(served.status).toBe(400);
		expect(arrived).toEqual([]);
	});

	test('a body that resolves nobody is refused rather than asked anonymously', async () => {
		const served = await servePublicAPIRequest(dispatchReachingTheSocket(), { ...invoke, requester: '' });

		expect(served.status).toBe(400);
		expect(arrived).toEqual([]);
	});

	test('a read is asked for at the path and query the caller used', async () => {
		await servePublicAPIRequest(dispatchReachingTheSocket(), {
			...invoke,
			method: 'GET',
			path: '/tasks',
			query: '?limit=20&state=open',
			payload: undefined
		});

		expect(arrived[0]?.method).toBe('GET');
		expect(arrived[0]?.path).toBe('/api/v1/tasks');
		expect(arrived[0]?.search).toBe('?limit=20&state=open');
		expect(arrived[0]?.body).toBe('');
	});

	test('a read carries no body, whatever payload came with it', async () => {
		await servePublicAPIRequest(dispatchReachingTheSocket(), {
			...invoke,
			method: 'get',
			path: '/tasks',
			payload: { limit: 20 }
		});

		expect(arrived[0]?.method).toBe('GET');
		expect(arrived[0]?.body).toBe('');
	});
});

describe('publicAPIRequestOf', () => {
	const invoke = {
		method: 'POST',
		path: '/tools/message_send/invoke',
		query: '',
		permission: 'write',
		requester: 'sample@example.test',
		payload: {}
	};

	test('reads the six fields the contract names and nothing else', () => {
		expect(publicAPIRequestOf({ ...invoke, headers: { Authorization: 'Bearer somebody-elses-key' } })).toEqual({
			method: 'POST',
			path: '/tools/message_send/invoke',
			query: '',
			permission: 'write',
			requester: 'sample@example.test',
			payload: {}
		});
	});

	test('a path that is not a path is nothing to ask for', () => {
		expect(publicAPIRequestOf({ ...invoke, path: 'tools/message_send/invoke' })).toBeNull();
		expect(publicAPIRequestOf({ ...invoke, path: '' })).toBeNull();
		expect(publicAPIRequestOf({ ...invoke, method: '' })).toBeNull();
		expect(publicAPIRequestOf({ ...invoke, permission: '' })).toBeNull();
	});

	test('a query is asked for with its question mark either way', () => {
		expect(publicAPIRequestOf({ ...invoke, query: 'limit=20' })?.query).toBe('?limit=20');
		expect(publicAPIRequestOf({ ...invoke, query: '?limit=20' })?.query).toBe('?limit=20');
		expect(publicAPIRequestOf({ ...invoke, query: '' })?.query).toBe('');
	});
});

describe('admindAPIURL', () => {
	const read: PublicAPIRequest = {
		method: 'GET',
		path: '/tasks',
		query: '?limit=20',
		permission: 'read',
		requester: 'sample@example.test',
		payload: undefined
	};

	test('the public API is asked for under /api/v1 on the socket', () => {
		expect(admindAPIURL(read)).toBe('http://internkim/api/v1/tasks?limit=20');
		expect(admindAPIURL({ ...read, query: '' })).toBe('http://internkim/api/v1/tasks');
	});
});

describe('a file the plane already kept in the company bucket', () => {
	const digest = 'a'.repeat(64);
	const kept = {
		requester: 'sample@example.test',
		permission: 'write',
		digest,
		contentType: 'image/png',
		filename: 'mascot.png'
	};

	function dispatchWithHome(agentPath: string | null) {
		const { dispatch, admindCalls, transfers } = dispatchThatKnows({});
		return {
			admindCalls,
			transfers,
			dispatch: {
				...dispatch,
				askAdmindAsRequester: async (call: AdmindCall) => {
					admindCalls.push(call);
					return { status: 200, body: rootsOf(agentPath) };
				}
			}
		};
	}

	function rootsOf(agentPath: string | null): { roots: unknown[] } {
		if (!agentPath) return { roots: [] };
		return {
			roots: [
				{ id: 'personal', kind: 'personal', agentPath },
				{ id: 'public', kind: 'public', agentPath: '/workspace/shared/public' }
			]
		};
	}

	function writtenPathsOf(transfers: { body: unknown }[]): string[] {
		return transfers.map((one) => (one.body as { path: string }).path);
	}

	test('is read as a digest, a content type and who asked, and never as a destination', () => {
		expect(publicAPIFileOf({ ...kept, path: '/workspace/etc' })).toEqual(kept);
		expect(publicAPIFileOf({ requester: 'sample@example.test', permission: 'write', digest })).toEqual({
			requester: 'sample@example.test',
			permission: 'write',
			digest,
			contentType: 'application/octet-stream',
			filename: ''
		});
	});

	test('is nothing to write when nobody, no permission, or no digest is named', () => {
		expect(publicAPIFileOf({ ...kept, digest: 'nope' })).toBeNull();
		expect(publicAPIFileOf({ ...kept, digest: `${digest}/../x` })).toBeNull();
		expect(publicAPIFileOf({ ...kept, requester: '' })).toBeNull();
		expect(publicAPIFileOf({ ...kept, permission: '' })).toBeNull();
	});

	test('is written into the requester own home, under the inbox a file arrives in', async () => {
		const { dispatch, transfers } = dispatchWithHome('/workspace/private/people/person-1');

		const served = await servePublicAPIFile(dispatch, kept);

		expect(served.status).toBe(200);
		expect(served.body).toEqual({
			file: {
				path: '/workspace/private/people/person-1/inbox/api/mascot.png',
				sizeBytes: 'kept bytes'.length,
				digest,
				contentType: 'image/png'
			}
		});
		expect(transfers).toEqual([
			{
				operation: 'writeKeptFileIntoWorkspace',
				memberID: '',
				requester: 'sample@example.test',
				body: { kept, path: '/workspace/private/people/person-1/inbox/api/mascot.png' }
			}
		]);
	});

	test('carries the leaf of the name offered, so a caller cannot climb out of the directory', async () => {
		const { dispatch, transfers } = dispatchWithHome('/workspace/private/people/person-1');

		await servePublicAPIFile(dispatch, { ...kept, filename: '../../../etc/passwd' });

		expect(writtenPathsOf(transfers)).toEqual(['/workspace/private/people/person-1/inbox/api/passwd']);
	});

	test('takes the digest and its extension when the caller offered no name', async () => {
		const { dispatch, transfers } = dispatchWithHome('/workspace/private/people/person-1');

		await servePublicAPIFile(dispatch, { ...kept, filename: '   ' });

		expect(writtenPathsOf(transfers)).toEqual([`/workspace/private/people/person-1/inbox/api/${digest}.png`]);
	});

	test('is refused when the address the key resolved to has no home, before anything is uploaded', async () => {
		const { dispatch, admindCalls } = dispatchWithHome(null);

		const served = await servePublicAPIFile(dispatch, { ...kept, requester: 'stranger@example.test' });

		expect(served.status).toBe(409);
		expect(admindCalls).toHaveLength(1);
	});

	test('is refused before admind is asked anything when the call names no file', async () => {
		const { dispatch, admindCalls } = dispatchWithHome('/workspace/private/people/person-1');

		const served = await servePublicAPIFile(dispatch, { requester: 'sample@example.test' });

		expect(served.status).toBe(400);
		expect(admindCalls).toEqual([]);
	});

	test('answers what the workspace answered when it refuses the write', async () => {
		const { dispatch } = dispatchWithHome('/workspace/private/people/person-1');
		const refusing = {
			...dispatch,
			transfer: {
				...dispatch.transfer,
				writeKeptFileIntoWorkspace: async () => {
					throw new TransferFailed(403, 'workspace path is not accessible');
				}
			}
		};

		const served = await servePublicAPIFile(refusing, kept);

		expect(served).toEqual({ status: 403, body: { error: 'workspace path is not accessible' } });
	});
});

describe('the name a kept file takes in the workspace', () => {
	test('is the leaf of what was offered, never a directory the caller chose', () => {
		expect(leafNameOf('report.pdf')).toBe('report.pdf');
		expect(leafNameOf('../../etc/passwd')).toBe('passwd');
		expect(leafNameOf('C:\\Users\\someone\\report.pdf')).toBe('report.pdf');
	});

	test('is nothing when nothing usable was offered', () => {
		for (const offered of ['', '   ', '.', '..', '.blueclaw', 'dir/']) expect(leafNameOf(offered)).toBe('');
	});
});

describe('a workspace file, over the socket admind listens on', () => {
	const arrived: { method: string; path: string; search: string; headers: Record<string, string>; content: string }[] = [];
	let socketPath = '';
	let admind: ReturnType<typeof Bun.serve> | null = null;

	beforeEach(() => {
		arrived.length = 0;
		socketPath = join(tmpdir(), `relay-file-${crypto.randomUUID().slice(0, 8)}.sock`);
		admind = Bun.serve({
			unix: socketPath,
			fetch: async (request) => {
				const asked = new URL(request.url);
				const headers: Record<string, string> = {};
				request.headers.forEach((value, name) => {
					headers[name] = value;
				});
				arrived.push({ method: request.method, path: asked.pathname, search: asked.search, headers, content: await request.text() });
				if (request.method === 'GET') {
					return new Response('2345', { status: 206, headers: { 'content-range': 'bytes 2-5/10' } });
				}
				return Response.json({ path: asked.searchParams.get('path'), sizeBytes: 12 });
			}
		});
	});

	afterEach(() => {
		admind?.stop(true);
		admind = null;
		rmSync(socketPath, { force: true });
	});

	test('is written by streaming its bytes to the exact path, under the two headers only the relay writes', async () => {
		const body = new ReadableStream<Uint8Array>({
			start(controller) {
				controller.enqueue(new TextEncoder().encode('report-'));
				controller.enqueue(new TextEncoder().encode('bytes'));
				controller.close();
			}
		});

		const response = await askAdmind(socketPath, {
			method: 'PUT',
			url: workspaceFileURL('/workspace/private/people/person-1/inbox/api/report.pdf'),
			requester: 'sample@example.test',
			permission: 'write',
			body
		});

		expect(response.status).toBe(200);
		expect(arrived[0]).toMatchObject({
			method: 'PUT',
			path: '/files/api/file',
			search: '?path=%2Fworkspace%2Fprivate%2Fpeople%2Fperson-1%2Finbox%2Fapi%2Freport.pdf',
			content: 'report-bytes'
		});
		expect(arrived[0]?.headers['x-internkim-requester-email']).toBe('sample@example.test');
		expect(arrived[0]?.headers['x-internkim-requester-permission']).toBe('write');
	});

	test('is read one range at a time, as the person who asked', async () => {
		const response = await askAdmind(socketPath, {
			method: 'GET',
			url: workspaceDownloadURL('/workspace/private/people/person-1/digits.txt'),
			requester: 'sample@example.test',
			range: 'bytes=2-5'
		});

		expect(response.status).toBe(206);
		expect(await response.text()).toBe('2345');
		expect(arrived[0]?.headers.range).toBe('bytes=2-5');
		expect(arrived[0]?.headers['x-internkim-requester-email']).toBe('sample@example.test');
	});
});

describe('a workspace call from a member, over the socket admind listens on', () => {
	const arrived: ArrivedRequest[] = [];
	let socketPath = '';
	let admind: ReturnType<typeof admindListeningOn> | null = null;

	beforeEach(() => {
		arrived.length = 0;
		socketPath = join(tmpdir(), `relay-workspace-${crypto.randomUUID().slice(0, 8)}.sock`);
		admind = admindListeningOn(socketPath, arrived);
	});

	afterEach(() => {
		admind?.stop(true);
		admind = null;
		rmSync(socketPath, { force: true });
	});

	test('reaches admind as the person who asked, where a requester header is heard at all', async () => {
		const { dispatch } = dispatchThatKnows({});

		const served = await serveCallForMember(
			{ ...dispatch, askAdmindAsRequester: (call: AdmindCall) => forwardToAdmind(socketPath, call) },
			{ callID: 'c1', capability: 'person.runs.list', body: {} },
			'member-1'
		);

		expect(served.status).toBe(200);
		expect(arrived).toHaveLength(1);
		expect(arrived[0]?.method).toBe('GET');
		expect(arrived[0]?.path).toBe('/runs/api');
		expect(arrived[0]?.headers['x-internkim-requester-email']).toBe('sample@example.test');
		expect(arrived[0]?.headers['x-internkim-requester-permission']).toBeUndefined();
	});
});

describe('a telling the plane sends as the bot', () => {
	test('is carried to admind, without asking which messenger credential anybody holds', async () => {
		const { dispatch, admindCalls } = dispatchThatKnows({});
		const withoutACredential = { ...dispatch, messengerCredentialOf: async () => null };

		const served = await serveTelling(withoutACredential, {
			recipientEmail: 'Sample@Example.test',
			message: '결재를 기다리는 건이 있습니다'
		});

		expect(served.status).toBe(200);
		expect(admindCalls).toHaveLength(1);
		expect(admindCalls[0]).toMatchObject({
			method: 'POST',
			url: 'http://internkim/tell/api/direct-message',
			requester: 'sample@example.test',
			contentType: 'application/json'
		});
		expect(JSON.parse(String(admindCalls[0]?.body))).toEqual({
			recipientEmail: 'sample@example.test',
			message: '결재를 기다리는 건이 있습니다'
		});
	});

	test('names a recipient and carries a message, or it is nothing to deliver', () => {
		expect(tellCallOf({ recipientEmail: 'sample@example.test', message: '   ' })).toBeNull();
		expect(tellCallOf({ recipientEmail: '', message: '안녕하세요' })).toBeNull();
		expect(tellCallOf({ recipientEmail: 'sample@example.test\nX: y', message: '안녕하세요' })).toBeNull();
	});

	test('is refused before admind hears of it when it names nobody', async () => {
		const { dispatch, admindCalls } = dispatchThatKnows({});

		const served = await serveTelling(dispatch, { message: '안녕하세요' });

		expect(served.status).toBe(400);
		expect(admindCalls).toHaveLength(0);
	});
});
