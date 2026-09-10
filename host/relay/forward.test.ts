import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import {
	admindAPIURL,
	answerBodyOf,
	apiRequestCapability,
	forwardToAdmind,
	forwardToAdmindAPI,
	forwardToChatd,
	isPersonCapability,
	isRegistrationCapability,
	isWorkspaceCapability,
	leafNameOf,
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
	workspaceUploadURL,
	type AdmindCall,
	type ConnectedAccount,
	type KeptFileReference,
	type PublicAPIRequest
} from './forward';
import type { ArrivedMessage } from './arrived';
import { rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

function dispatchThatKnows(externalIDs: Record<string, string>) {
	const asked: { capability: string; body: Record<string, unknown> }[] = [];
	const connected: { memberID: string; account: ConnectedAccount }[] = [];
	const admindCalls: AdmindCall[] = [];
	const arrivals: { conversationID: string; messageID: string }[] = [];
	const told: ArrivedMessage[] = [];
	return {
		asked,
		connected,
		admindCalls,
		arrivals,
		told,
		dispatch: {
			tellThoseAddressed: async (arrived: ArrivedMessage) => {
				told.push(arrived);
				return arrived.recipientExternalIDs.length;
			},
			connectMessengerAccount: async (memberID: string, account: ConnectedAccount) => {
				connected.push({ memberID, account });
			},
			serveAsset: async (capability: string) => ({ served: capability }),
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
			keepAttachment: async (contentBase64: string) => ({
				address: keptAddress(contentBase64),
				sizeBytes: Buffer.from(contentBase64, 'base64').byteLength,
				digest: contentBase64
			}),
			keptAlready: async () => null,
			keptFileBytes: async () => new TextEncoder().encode('kept bytes'),
			askAdmindAsRequester: async (call: AdmindCall) => {
				admindCalls.push(call);
				return { status: 200, body: { served: call.url } };
			},
			largestFileBytes: 200_000_000
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

function dispatchThatRefuses(refusedAttachments: { index: number; filename: string }[]) {
	const { asked, arrivals, dispatch } = dispatchThatKnows({});
	let sends = 0;
	return {
		asked,
		arrivals,
		dispatch: {
			...dispatch,
			askChatd: async (capability: string, body: Record<string, unknown>) => {
				asked.push({ capability, body });
				if (capability !== 'person.message.send') return { status: 200, body: {} };
				sends += 1;
				if (sends > 1) return { status: 200, body: { id: 'event-1' } };
				return {
					status: 415,
					body: {
						error: 'the store refused them',
						refusedAttachments: refusedAttachments.map((one) => ({
							...one,
							status: 415,
							reason: 'disallowed content type'
						}))
					}
				};
			}
		}
	};
}

function dispatchHolding(file: { filename: string; contentType: string; contentBase64: string } | null) {
	const { asked, dispatch } = dispatchThatKnows({});
	return {
		asked,
		dispatch: {
			...dispatch,
			askChatd: async (capability: string, body: Record<string, unknown>, largestBytes?: number) => {
				asked.push({ capability, body: { ...body, largestBytes } });
				return { status: 200, body: { file } };
			}
		}
	};
}

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
	function dispatchThatDelivers() {
		const { asked, arrivals, told, dispatch } = dispatchThatKnows({});
		return {
			told,
			arrivals,
			dispatch: {
				...dispatch,
				askChatd: async (capability: string, body: Record<string, unknown>) => {
					asked.push({ capability, body });
					if (capability === 'person.message.send') return { status: 200, body: { id: 'event-1' } };
					if (capability === 'person.identity') return { status: 200, body: { externalID: 'U-author' } };
					return {
						status: 200,
						body: { conversations: [{ id: 'channel-1', participantExternalIDs: ['U-author', 'U-first'] }] }
					};
				}
			}
		};
	}

	test('is told to the others in the conversation once the messenger took it', async () => {
		const { told, arrivals, dispatch } = dispatchThatDelivers();

		const served = await serveCallForMember(
			dispatch,
			{ callID: 'c1', capability: 'person.message.send', body: { conversationID: 'channel-1', body: '지금 갈게요' } },
			'member-1'
		);
		await untilTold(told);

		expect(served.status).toBe(200);
		expect(arrivals).toEqual([{ conversationID: 'channel-1', messageID: 'event-1' }]);
		expect(told).toEqual([
			{
				conversationID: 'channel-1',
				messageID: 'event-1',
				authorExternalID: 'U-author',
				authorName: '',
				recipientExternalIDs: ['U-first'],
				preview: '지금 갈게요'
			}
		]);
	});

	test('a message the messenger refused is told to nobody', async () => {
		const { told, dispatch } = dispatchThatDelivers();

		const served = await serveCallForMember(
			{
				...dispatch,
				askChatd: async () => ({ status: 403, body: { error: 'not in this channel' } })
			},
			{ callID: 'c1', capability: 'person.message.send', body: { conversationID: 'channel-1', body: '지금 갈게요' } },
			'member-1'
		);
		await untilTold(told, 0);

		expect(served.status).toBe(403);
		expect(told).toEqual([]);
	});
});

async function untilTold(told: ArrivedMessage[], expected = 1): Promise<void> {
	for (let turn = 0; turn < 20 && told.length < expected; turn += 1) {
		await new Promise((settle) => setTimeout(settle, 1));
	}
}

describe('a message carrying a file the messenger will not store', () => {
	const send = {
		callID: 'c1',
		capability: 'person.message.send',
		body: {
			conversationID: 'channel-1',
			body: 'here it is',
			attachments: [
				{ filename: 'notes.pdf', contentType: 'application/pdf', contentBase64: 'AAAA' },
				{ filename: 'page.html', contentType: 'text/html', contentBase64: 'BBBB' }
			]
		}
	};

	test('is sent again with that file kept where the company can read it', async () => {
		const { asked, arrivals, dispatch } = dispatchThatRefuses([{ index: 1, filename: 'page.html' }]);

		const served = await serveCallForMember(dispatch, send, 'member-1');

		expect(served.status).toBe(200);
		expect(arrivals).toEqual([{ conversationID: 'channel-1', messageID: 'event-1' }]);

		const sent = asked.filter((entry) => entry.capability === 'person.message.send');
		expect(sent).toHaveLength(2);
		expect(sent[1]?.body.attachments).toEqual([
			{ filename: 'notes.pdf', contentType: 'application/pdf', contentBase64: 'AAAA' },
			{
				filename: 'page.html',
				contentType: 'text/html',
				address: 'https://company.supabase.co/storage/v1/object/asset/company-1/shared/attachment/BBBB',
				sizeBytes: 3,
				digest: 'BBBB'
			}
		]);
	});

	test('the files it did store are not kept a second time', async () => {
		const kept: string[] = [];
		const { dispatch } = dispatchThatRefuses([{ index: 1, filename: 'page.html' }]);

		await serveCallForMember(
			{
				...dispatch,
				keepAttachment: async (contentBase64: string) => {
					kept.push(contentBase64);
					return { address: 'https://company.supabase.co/a', sizeBytes: 3, digest: 'a' };
				}
			},
			send,
			'member-1'
		);

		expect(kept).toEqual(['BBBB']);
	});

	test('a message the messenger took whole is sent once', async () => {
		const { asked, dispatch } = dispatchThatKnows({});

		const served = await serveCallForMember(dispatch, send, 'member-1');

		expect(served.status).toBe(200);
		expect(asked.filter((entry) => entry.capability === 'person.message.send')).toHaveLength(1);
	});

	test('a refusal that names no file is handed back as it came', async () => {
		const { asked, dispatch } = dispatchThatRefuses([]);

		const served = await serveCallForMember(dispatch, send, 'member-1');

		expect(served.status).toBe(415);
		expect(asked.filter((entry) => entry.capability === 'person.message.send')).toHaveLength(1);
	});
});

describe('opening a file the messenger holds on this machine', () => {
	const read = {
		callID: 'c1',
		capability: 'person.message.attachment',
		body: {
			messageID: 'http://localhost:3000/media/9f2c.pdf',
			filename: '2026 예산.pdf',
			contentType: 'application/pdf',
			digest: '9f2c'
		}
	};
	const file = { filename: '9f2c.pdf', contentType: 'application/pdf', contentBase64: 'AAAA' };

	test('is answered with an address in the company bucket, never with the file', async () => {
		const { dispatch } = dispatchHolding(file);

		const served = await serveCallForMember(dispatch, read, 'member-1');

		expect(served.status).toBe(200);
		expect(served.body).toEqual({
			attachment: {
				address: keptAddress('AAAA'),
				sizeBytes: 3,
				digest: 'AAAA',
				filename: '9f2c.pdf',
				contentType: 'application/pdf'
			}
		});
	});

	test('one already kept is answered without the messenger being asked at all', async () => {
		const { asked, dispatch } = dispatchHolding(file);

		const served = await serveCallForMember(
			{
				...dispatch,
				keptAlready: async (digest: string) => ({
					address: keptAddress(digest),
					sizeBytes: 91_000_000,
					digest
				})
			},
			read,
			'member-1'
		);

		expect(asked).toEqual([]);
		expect(served.body).toEqual({
			attachment: {
				address: keptAddress('9f2c'),
				sizeBytes: 91_000_000,
				digest: '9f2c',
				filename: '2026 예산.pdf',
				contentType: 'application/pdf'
			}
		});
	});

	test('a message that names no hash is read rather than guessed at', async () => {
		const { asked, dispatch } = dispatchHolding(file);
		let askedFor = '';

		await serveCallForMember(
			{
				...dispatch,
				keptAlready: async (digest: string) => {
					askedFor = digest;
					return null;
				}
			},
			{ ...read, body: { ...read.body, digest: '' } },
			'member-1'
		);

		expect(askedFor).toBe('');
		expect(asked).toHaveLength(1);
	});

	test('the messenger is asked for the whole file, not what a picture may weigh', async () => {
		const { asked, dispatch } = dispatchHolding(file);

		await serveCallForMember(dispatch, read, 'member-1');

		expect(asked[0]?.body.largestBytes).toBe(200_000_000);
	});

	test('a file the messenger will not serve is answered as no attachment', async () => {
		const { dispatch } = dispatchHolding(null);

		const served = await serveCallForMember(dispatch, read, 'member-1');

		expect(served.body).toEqual({ attachment: null });
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
			{ taskRunID: 'run-1', decision: 'confirm', actor: { kind: 'buzz' } },
			'sample@example.test'
		);

		expect(call?.method).toBe('POST');
		expect(call?.url).toBe('http://internkim/runs/api/approve');
		expect(call?.contentType).toBe('application/json');
		expect(JSON.parse(String(call?.body))).toEqual({ taskRunID: 'run-1', decision: 'confirm' });
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
		const { dispatch, admindCalls } = dispatchThatKnows({});
		return {
			admindCalls,
			dispatch: {
				...dispatch,
				askAdmindAsRequester: async (call: AdmindCall) => {
					admindCalls.push(call);
					if (call.url.includes('/files/api/roots')) return { status: 200, body: rootsOf(agentPath) };
					return { status: 200, body: { uploaded: [uploadedNameOf(call)] } };
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

	function uploadedNameOf(call: AdmindCall): string {
		const part = call.body instanceof FormData ? call.body.get('file') : null;
		return part instanceof File ? part.name : '';
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

	test('is uploaded to admind, into the requester own home, under the inbox a file arrives in', async () => {
		const { dispatch, admindCalls } = dispatchWithHome('/workspace/private/people/person-1');

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
		expect(admindCalls[1]).toMatchObject({
			method: 'POST',
			url: workspaceUploadURL('/workspace/private/people/person-1/inbox/api'),
			requester: 'sample@example.test',
			permission: 'write'
		});
	});

	test('is asked for under the name admind reports back, never the one the relay offered', async () => {
		const { dispatch, admindCalls } = dispatchWithHome('/workspace/private/people/person-1');
		const renaming = {
			...dispatch,
			askAdmindAsRequester: async (call: AdmindCall) => {
				const answer = await dispatch.askAdmindAsRequester(call);
				if (call.url.includes('/files/api/roots')) return answer;
				return { status: 200, body: { uploaded: ['mascot-1.png'] } };
			}
		};

		const served = await servePublicAPIFile(renaming, kept);

		expect(served.body).toEqual({
			file: {
				path: '/workspace/private/people/person-1/inbox/api/mascot-1.png',
				sizeBytes: 'kept bytes'.length,
				digest,
				contentType: 'image/png'
			}
		});
		expect(admindCalls).toHaveLength(2);
	});

	test('carries the leaf of the name offered, so a caller cannot climb out of the directory', async () => {
		const { dispatch, admindCalls } = dispatchWithHome('/workspace/private/people/person-1');

		const served = await servePublicAPIFile(dispatch, { ...kept, filename: '../../../etc/passwd' });

		expect(served.body).toEqual({
			file: {
				path: '/workspace/private/people/person-1/inbox/api/passwd',
				sizeBytes: 'kept bytes'.length,
				digest,
				contentType: 'image/png'
			}
		});
		expect(admindCalls[1]?.url).toBe(
			workspaceUploadURL('/workspace/private/people/person-1/inbox/api')
		);
	});

	test('takes the digest and its extension when the caller offered no name', async () => {
		const { dispatch, admindCalls } = dispatchWithHome('/workspace/private/people/person-1');

		const served = await servePublicAPIFile(dispatch, { ...kept, filename: '   ' });

		expect(served.body).toMatchObject({
			file: { path: `/workspace/private/people/person-1/inbox/api/${digest}.png` }
		});
		expect(admindCalls).toHaveLength(2);
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

	test('answers what admind answered when admind refuses the write', async () => {
		const { dispatch, admindCalls } = dispatchWithHome('/workspace/private/people/person-1');
		const refusing = {
			...dispatch,
			askAdmindAsRequester: async (call: AdmindCall) => {
				const answer = await dispatch.askAdmindAsRequester(call);
				if (call.url.includes('/files/api/roots')) return answer;
				return { status: 403, body: { error: 'workspace path is not accessible' } };
			}
		};

		const served = await servePublicAPIFile(refusing, kept);

		expect(served).toEqual({ status: 403, body: { error: 'workspace path is not accessible' } });
		expect(admindCalls).toHaveLength(2);
	});

	test('answers a failure when admind takes the call and writes nothing', async () => {
		const { dispatch } = dispatchWithHome('/workspace/private/people/person-1');
		const writingNothing = {
			...dispatch,
			askAdmindAsRequester: async (call: AdmindCall) => {
				const answer = await dispatch.askAdmindAsRequester(call);
				if (call.url.includes('/files/api/roots')) return answer;
				return { status: 200, body: { uploaded: [] } };
			}
		};

		expect((await servePublicAPIFile(writingNothing, kept)).status).toBe(502);
	});
});

describe('the name a kept file takes in the workspace', () => {
	test('is the leaf of what was offered, never a directory the caller chose', () => {
		expect(leafNameOf('report.pdf', 'fallback')).toBe('report.pdf');
		expect(leafNameOf('../../etc/passwd', 'fallback')).toBe('passwd');
		expect(leafNameOf('C:\\Users\\someone\\report.pdf', 'fallback')).toBe('report.pdf');
	});

	test('is the fallback when nothing usable was offered', () => {
		expect(leafNameOf('', 'digest.png')).toBe('digest.png');
		expect(leafNameOf('   ', 'digest.png')).toBe('digest.png');
		expect(leafNameOf('..', 'digest.png')).toBe('digest.png');
		expect(leafNameOf('/.blueclaw', 'digest.png')).toBe('digest.png');
	});
});

describe('the upload admind is asked for', () => {
	test('names the directory the relay derived, as a query it cannot be mistaken for a path', () => {
		expect(workspaceUploadURL('/workspace/private/people/person-1/inbox/api')).toBe(
			'http://internkim/files/api/upload?path=%2Fworkspace%2Fprivate%2Fpeople%2Fperson-1%2Finbox%2Fapi'
		);
	});
});

describe('the materialising capability, over the socket admind listens on', () => {
	const digest = 'b'.repeat(64);
	const uploads: { path: string; search: string; headers: Record<string, string>; filename: string; content: string }[] = [];
	let socketPath = '';
	let admind: ReturnType<typeof Bun.serve> | null = null;

	beforeEach(() => {
		uploads.length = 0;
		socketPath = join(tmpdir(), `relay-upload-${crypto.randomUUID().slice(0, 8)}.sock`);
		admind = Bun.serve({
			unix: socketPath,
			fetch: async (request) => {
				const asked = new URL(request.url);
				if (asked.pathname === '/files/api/roots') {
					return Response.json({
						roots: [{ id: 'personal', kind: 'personal', agentPath: '/workspace/private/people/person-1' }]
					});
				}
				const headers: Record<string, string> = {};
				request.headers.forEach((value, name) => {
					headers[name] = value;
				});
				const part = (await request.formData()).get('file');
				const file = part instanceof File ? part : new File([], '');
				uploads.push({
					path: asked.pathname,
					search: asked.search,
					headers,
					filename: file.name,
					content: await file.text()
				});
				return Response.json({ uploaded: [file.name] });
			}
		});
	});

	afterEach(() => {
		admind?.stop(true);
		admind = null;
		rmSync(socketPath, { force: true });
	});

	test('posts the bytes as one multipart part, under the two headers only the relay writes', async () => {
		const { dispatch } = dispatchThatKnows({});

		const served = await servePublicAPIFile(
			{ ...dispatch, askAdmindAsRequester: (call: AdmindCall) => forwardToAdmind(socketPath, call) },
			{
				requester: 'sample@example.test',
				permission: 'write',
				digest,
				contentType: 'text/plain',
				filename: 'notes.txt'
			}
		);

		expect(served.status).toBe(200);
		expect(served.body).toEqual({
			file: {
				path: '/workspace/private/people/person-1/inbox/api/notes.txt',
				sizeBytes: 'kept bytes'.length,
				digest,
				contentType: 'text/plain'
			}
		});
		expect(uploads).toHaveLength(1);
		expect(uploads[0]?.path).toBe('/files/api/upload');
		expect(uploads[0]?.search).toBe('?path=%2Fworkspace%2Fprivate%2Fpeople%2Fperson-1%2Finbox%2Fapi');
		expect(uploads[0]?.filename).toBe('notes.txt');
		expect(uploads[0]?.content).toBe('kept bytes');
		expect(uploads[0]?.headers['x-internkim-requester-email']).toBe('sample@example.test');
		expect(uploads[0]?.headers['x-internkim-requester-permission']).toBe('write');
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
