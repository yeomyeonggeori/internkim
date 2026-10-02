import { expect, test } from 'bun:test';
import { MessengerAddresses } from './messenger-address';

test('asks the record about an address once a minute, including the ones nobody holds', async () => {
	let now = 0;
	const asked: string[] = [];
	const addresses = new MessengerAddresses(
		async (slug) => {
			asked.push(slug);
			return slug === 'acme' ? 'c1' : null;
		},
		() => now
	);
	expect(await addresses.companyOf('acme')).toBe('c1');
	expect(await addresses.companyOf('acme')).toBe('c1');
	expect(await addresses.companyOf('nobody')).toBeNull();
	expect(await addresses.companyOf('nobody')).toBeNull();
	now = 60_001;
	expect(await addresses.companyOf('acme')).toBe('c1');
	expect(asked).toEqual(['acme', 'nobody', 'acme']);
});
