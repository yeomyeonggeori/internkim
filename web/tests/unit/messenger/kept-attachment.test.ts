import { describe, expect, test } from 'bun:test';
import {
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
