import { expect, test } from 'bun:test';
import { aCompanyPlane } from './a-company-plane';

test('a plane told two different messengers refuses to start', async () => {
	const refusal = aCompanyPlane({ messengerPlatform: 'buzz', disagreeAbout: 'capabilityd' });
	await expect(refusal).rejects.toThrow(/disagrees with itself/);
}, 120_000);
