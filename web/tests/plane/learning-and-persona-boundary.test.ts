import { afterAll, beforeAll, expect, test } from 'bun:test';
import { aCompanyPlane, type ACompanyPlane } from './a-company-plane';

let plane: ACompanyPlane;

beforeAll(async () => {
	plane = await aCompanyPlane();
}, 180_000);

afterAll(async () => {
	await plane?.stop();
});

async function requestAsMember(
	path: string,
	memberIndex = 0,
	options: RequestInit = {}
): Promise<Response> {
	const person = plane.people[memberIndex];
	return fetch(`http://plane${path}`, {
		...options,
		unix: plane.requesterSocketPath,
		headers: {
			...(options.headers ?? {}),
			'X-INTERNKIM-REQUESTER-EMAIL': person.email,
			'X-INTERNKIM-REQUESTER-PERMISSION': 'read'
		}
	});
}

test('company learning and persona routes enforce the requester boundary', async () => {
	const spoofed = await fetch(`${plane.admindURL}/agent-learning/api/skills`, {
		headers: { 'X-INTERNKIM-REQUESTER-EMAIL': plane.people[0].email }
	});
	expect(spoofed.status).toBe(403);

	const skills = await requestAsMember('/agent-learning/api/skills?includeRetired=true');
	expect(skills.status, await skills.clone().text()).toBe(200);
	const skillDocument = (await skills.json()) as {
		settings?: { enabled?: boolean };
		skills?: unknown[];
	};
	expect(skillDocument.settings?.enabled).toBe(false);
	expect(Array.isArray(skillDocument.skills)).toBe(true);

	const settings = await requestAsMember('/agent-learning/api/settings');
	expect(settings.status, await settings.clone().text()).toBe(200);
	expect(await settings.json()).toMatchObject({ enabled: false });

	for (const path of ['/agent-learning/api/soul', '/agent-learning/api/soul/history']) {
		const soul = await requestAsMember(path);
		expect(soul.status, `${path}: ${await soul.clone().text()}`).toBe(200);
	}

	const soulWrite = await requestAsMember('/agent-learning/api/soul', 0, {
		method: 'POST',
		body: '{}',
		headers: { 'Content-Type': 'application/json' }
	});
	expect(soulWrite.status).toBe(405);

	const memberSettingsWrite = await requestAsMember('/agent-learning/api/settings', 1, {
		method: 'POST',
		body: JSON.stringify({ enabled: true, activeLimit: 20 }),
		headers: { 'Content-Type': 'application/json' }
	});
	expect(memberSettingsWrite.status).toBe(403);

	const identity = await requestAsMember('/persona/api/identity');
	expect(identity.status, await identity.clone().text()).toBe(200);
});
