import { afterEach, describe, expect, test } from 'bun:test';
import {
	answerBodyOf,
	forwardToChatd,
	isPersonCapability,
	isRegistrationCapability,
	isWorkspaceCapability,
	mailOperationOf,
	serveCallForMember,
	type ConnectedAccount
} from './forward';

function dispatchThatKnows(externalIDs: Record<string, string>) {
	const asked: { capability: string; body: Record<string, unknown> }[] = [];
	const connected: { memberID: string; account: ConnectedAccount }[] = [];
	return {
		asked,
		connected,
		dispatch: {
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
			askAdmind: async (capability: string, body: Record<string, unknown>, requesterEmail: string) => {
				asked.push({ capability, body: { ...body, requesterEmail } });
				return { status: 200, body: { served: capability } };
			},
			emailOfMember: async (memberID: string) =>
				memberID === 'member-1' ? 'sample@example.test' : null,
			messengerCredentialOf: async () => ({ kind: 'buzz-token', secret: 'a-held-secret' }),
			memberOfExternalID: async (externalID: string) => externalIDs[externalID] ?? null,
			keepAttachment: async (contentBase64: string) => ({
				address: keptAddress(contentBase64),
				sizeBytes: Buffer.from(contentBase64, 'base64').byteLength,
				digest: contentBase64
			}),
			keptAlready: async () => null,
			largestFileBytes: 200_000_000
		}
	};
}

function keptAddress(digest: string): string {
	return `https://company.supabase.co/storage/v1/object/asset/company-1/shared/attachment/${digest}`;
}

function dispatchThatRefuses(refusedAttachments: { index: number; filename: string }[]) {
	const { asked, dispatch } = dispatchThatKnows({});
	let sends = 0;
	return {
		asked,
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
		const { asked, dispatch } = dispatchThatKnows({ 'U-known': 'member-1' });

		const served = await serveCallForMember(dispatch, {
			callID: 'c1',
			capability: 'person.memory.graph',
			body: {}
		}, 'member-1');

		expect(served.status).toBe(200);
		expect(served.replyTo).toBe('member-1');
		const workspaceCall = asked.find((entry) => entry.capability === 'person.memory.graph');
		expect(workspaceCall?.body.requesterEmail).toBe('sample@example.test');
	});

	test('a workspace call for a member the record has no address for is refused', async () => {
		const { dispatch } = dispatchThatKnows({});

		const served = await serveCallForMember(dispatch, {
			callID: 'c1',
			capability: 'person.memory.graph',
			body: {}
		}, 'member-2');

		expect(served.status).toBe(409);
	});

});

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
		const { asked, dispatch } = dispatchThatRefuses([{ index: 1, filename: 'page.html' }]);

		const served = await serveCallForMember(dispatch, send, 'member-1');

		expect(served.status).toBe(200);
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
				account: { externalID: 'U-new', name: '이샘플', secret: 'a-durable-token' }
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
});

test('a message is still the messenger, not the workspace', () => {
	expect(isWorkspaceCapability('person.message.send')).toBe(false);
});
