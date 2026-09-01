import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { aCompanyPlane } from './a-company-plane';

const repositoryRoot = join(import.meta.dir, '..', '..', '..');

// The check that a plane makes about itself is worth exactly as much as the
// proof that it would refuse. So: hand it a disagreement and watch it refuse.
test('a plane told two different messengers refuses to start', async () => {
	const refusal = aCompanyPlane({ messengerPlatform: 'buzz', disagreeAbout: 'capabilityd' });
	await expect(refusal).rejects.toThrow(/disagrees with itself/);
}, 120_000);

// The sandbox starts the plane the company's own box starts. When the box starts
// something else, the sandbox is testing a plane nobody runs — so the two lists
// are compared rather than trusted.
test('the company box starts every process this sandbox does', () => {
	const entrypoint = readFileSync(join(repositoryRoot, 'host', 'entrypoint.sh'), 'utf8');
	const started = ['internkim-capabilityd', 'internkim-admind', 'blueclaw', 'chatd'];
	const missing = started.filter((process) => !entrypoint.includes(process));
	expect(
		missing,
		`host/entrypoint.sh starts no ${missing.join(', ')}. ` +
			`host/relay/relay.ts routes every workspace call to admind at :18080, so on a real ` +
			`company plane those calls answer 500 and a direct message cannot go out under a ` +
			`person's own name.`
	).toEqual([]);
});

// capabilityd chooses its messenger from these two flags alone. A box that omits
// them gets the Mattermost branch and a "sent" it did not earn.
test('the company box tells capabilityd which messenger it serves', () => {
	const entrypoint = readFileSync(join(repositoryRoot, 'host', 'entrypoint.sh'), 'utf8');
	const capabilitydBlock = entrypoint.slice(entrypoint.indexOf('internkim-capabilityd \\'));
	for (const flag of ['--chatd-endpoint', '--chatd-platform']) {
		expect(
			capabilitydBlock.includes(flag),
			`host/entrypoint.sh starts capabilityd without ${flag}, so chatdServesTheMessenger() is ` +
				`false and every message tool takes the Mattermost branch to http://localhost:8065`
		).toBe(true);
	}
});
