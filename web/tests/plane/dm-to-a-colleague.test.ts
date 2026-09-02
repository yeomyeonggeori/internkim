import { afterAll, beforeAll, expect, test } from 'bun:test';
import { aCompanyPlane, type ACompanyPlane } from './a-company-plane';
import { directMessagesDelivered, platformOf, postsDelivered } from './a-messenger-nobody-runs';

// 이샘플 asks the agent to write to 박예시. This is the shape a member's request
// takes through the internkim-api skill, and the shape that answered "sent" while
// the message left on a messenger nobody at the company reads.

let plane: ACompanyPlane;

beforeAll(async () => {
	plane = await aCompanyPlane({ messengerPlatform: 'buzz' });
}, 120_000);

afterAll(async () => {
	await plane?.stop();
});

async function askThePlaneToWrite(message: string): Promise<Response> {
	const [sender, recipient] = plane.people;
	// Over the requester socket, because that is the door the relay uses and the
	// only one the public API will take an asserted requester from.
	return fetch(`http://plane/api/v1/tools/message_send/invoke`, {
		unix: plane.requesterSocketPath,
		method: 'POST',
		headers: {
			'Content-Type': 'application/json',
			'X-INTERNKIM-REQUESTER-EMAIL': sender.email,
			'X-INTERNKIM-REQUESTER-PERMISSION': 'write'
		},
		body: JSON.stringify({
			input: {
				targetType: 'directMessage',
				personHint: recipient.name,
				message
			}
		})
	});
}

test('a direct message leaves on the messenger this company runs', async () => {
	const marker = `평면 점검 ${Date.now()}`;
	const answer = await askThePlaneToWrite(marker);
	// A status on its own sends the reader to a log. The plane says why it
	// refused, so the assertion carries what it said.
	expect(answer.status, `the plane refused: ${await answer.clone().text()}`).toBe(200);

	const wentToTheWrongMessenger = postsDelivered(plane.messenger);
	expect(
		wentToTheWrongMessenger,
		`the message went to the messenger nobody runs at ${plane.messenger.url}, not to ${plane.messengerPlatform}. ` +
			`capabilityd decides this with --chatd-platform and --chatd-endpoint`
	).toHaveLength(0);

	const delivered = directMessagesDelivered(plane.connector);
	expect(
		delivered.length,
		`nothing reached the messenger connector at all.\n` +
			`  it saw: ${JSON.stringify(plane.connector.pathsCalled())}\n` +
			`  the plane answered: ${await answer.clone().text()}`
	).toBeGreaterThan(0);
	expect(delivered.map(platformOf)).toContain(plane.messengerPlatform);
});

test('it carries what the person actually wrote', async () => {
	const marker = `평면 점검 본문 ${Date.now()}`;
	await askThePlaneToWrite(marker);

	const carried = directMessagesDelivered(plane.connector).some((call) =>
		JSON.stringify(call.body ?? '').includes(marker)
	);
	expect(carried, 'the connector was called but the message was not the one that was written').toBe(
		true
	);
});
