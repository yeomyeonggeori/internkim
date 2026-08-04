//   bun run web/scripts/claim-company-address.ts --slug <company slug> --served-from <hostname>

import { claimCompanyAddress } from '../src/lib/server/company-address';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const slug = argument('slug');
if (!slug) throw new Error('pass --slug <company slug>');

const address = await claimCompanyAddress(process.env, slug, argument('served-from') ?? '');
console.log(address.hostname ? `${address.hostname} — ${address.status || 'claimed'}` : address.status);
