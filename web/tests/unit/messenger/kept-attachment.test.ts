import { describe, expect, test } from 'bun:test';
import {
	addressesToSign,
	keptAssetPathOf,
	readableAddresses,
	type AttachmentSigner
} from '$lib/messenger/kept-attachment';

const projectURL = 'https://project.supabase.co';
const company = '43000000-0000-0000-0000-0000000000a0';
const addressOf = (path: string) => `${projectURL}/storage/v1/object/asset/${path}`;

function signerOver(known: string[]): AttachmentSigner & { asked: string[][] } {
	const asked: string[][] = [];
	return {
		asked,
		createSignedUrls: async (paths: string[]) => {
			asked.push(paths);
			return {
				data: paths.map((path) => ({
					path: known.includes(path) ? path : null,
					signedUrl: `${addressOf(path)}?token=signed`
				})),
				error: null
			};
		}
	};
}

describe('recognising a file kept in the company bucket', () => {
	test('reads the object path out of the address the message carries', () => {
		const path = `${company}/shared/attachment/abc.html`;
		expect(keptAssetPathOf(projectURL, addressOf(path))).toBe(path);
	});

	test('survives a project url written with a trailing slash', () => {
		expect(keptAssetPathOf(`${projectURL}/`, addressOf('a/b'))).toBe('a/b');
	});

	test('an address on the company machine is not one of ours', () => {
		expect(keptAssetPathOf(projectURL, 'http://localhost:3000/media/abc.png')).toBeNull();
	});

	test('a page not served by the central plane recognises nothing', () => {
		expect(keptAssetPathOf('', addressOf('a/b'))).toBeNull();
	});
});

describe('opening one', () => {
	test('the reader signs for it themselves', async () => {
		const path = `${company}/shared/attachment/abc.html`;
		const signer = signerOver([path]);

		const readable = await readableAddresses(signer, projectURL, [addressOf(path)]);

		expect(signer.asked).toEqual([[path]]);
		expect(readable.get(addressOf(path))).toBe(`${addressOf(path)}?token=signed`);
	});

	test('files the messenger stored itself are left alone', async () => {
		const signer = signerOver([]);

		const readable = await readableAddresses(signer, projectURL, ['http://localhost:3000/media/abc.png']);

		expect(signer.asked).toEqual([]);
		expect(readable.size).toBe(0);
	});

	test('one the record will not sign is left as it was rather than pointing nowhere', async () => {
		const mine = `${company}/shared/attachment/mine.html`;
		const theirs = 'other-company/shared/attachment/theirs.html';
		const signer = signerOver([mine]);

		const readable = await readableAddresses(signer, projectURL, [addressOf(mine), addressOf(theirs)]);

		expect([...readable.keys()]).toEqual([addressOf(mine)]);
	});

	test('a store that answers with an error changes nothing', async () => {
		const failing: AttachmentSigner = {
			createSignedUrls: async () => ({ data: null, error: { message: 'no' } })
		};

		const readable = await readableAddresses(failing, projectURL, [addressOf('a/b')]);

		expect(readable.size).toBe(0);
	});
});

describe('finding an address the browser can be given', () => {
	const onTheCompanyMachine = { url: 'http://localhost:3000/media/9f2c.pdf' };
	const kept = addressOf(`${company}/shared/attachment/9f2c.pdf`);

	test('a file on the company machine is copied to the bucket first', async () => {
		const asked: string[] = [];

		const addresses = await addressesToSign([onTheCompanyMachine], projectURL, async (attachment) => {
			asked.push(attachment.url);
			return { address: kept };
		});

		expect(asked).toEqual([onTheCompanyMachine.url]);
		expect(addresses.get(onTheCompanyMachine.url)).toBe(kept);
	});

	test('one already in the bucket is not copied again', async () => {
		let copies = 0;

		const addresses = await addressesToSign([{ url: kept }], projectURL, async () => {
			copies += 1;
			return { address: kept };
		});

		expect(copies).toBe(0);
		expect(addresses.get(kept)).toBe(kept);
	});

	test('a file the company server will not copy is left out rather than guessed at', async () => {
		const addresses = await addressesToSign([onTheCompanyMachine], projectURL, async () => null);

		expect(addresses.size).toBe(0);
	});

	test('one that fails does not take the rest of the page with it', async () => {
		const second = { url: 'http://localhost:3000/media/aaaa.png' };

		const addresses = await addressesToSign([onTheCompanyMachine, second], projectURL, async (attachment) => {
			if (attachment.url === onTheCompanyMachine.url) throw new Error('the company server is down');
			return { address: kept };
		});

		expect([...addresses.keys()]).toEqual([second.url]);
	});
});
