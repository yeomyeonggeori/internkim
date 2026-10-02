import { describe, expect, test } from 'bun:test';
import {
	attachmentAddress,
	digestOf,
	extensionOf,
	keepSharedAsset,
	sharedAssetKeptAs,
	sharedAssetPath,
	type AssetLister
} from './asset-store';

const company = '43000000-0000-0000-0000-0000000000a0';

describe('sharedAssetPath', () => {
	test('opens with the company, so the policy can read the owner off the path', async () => {
		const path = sharedAssetPath(company, 'attachment', await digestOf(new Uint8Array([1])), 'image/png');
		expect(path.startsWith(`${company}/shared/attachment/`)).toBe(true);
	});

	test('names the same bytes the same way, so the same picture is stored once', async () => {
		const bytes = new Uint8Array([1, 2, 3]);
		expect(sharedAssetPath(company, 'attachment', await digestOf(bytes), 'image/png')).toBe(
			sharedAssetPath(company, 'attachment', await digestOf(new Uint8Array([1, 2, 3])), 'image/png')
		);
	});

	test('names different bytes differently, so a changed picture busts its own cache', async () => {
		expect(await digestOf(new Uint8Array([1]))).not.toBe(await digestOf(new Uint8Array([2])));
	});
});

describe('extensionOf', () => {
	test('reads a type that carries parameters', () => {
		expect(extensionOf('image/png; charset=binary')).toBe('.png');
	});

	test('leaves a type it does not know without one, instead of guessing', () => {
		expect(extensionOf('application/octet-stream')).toBe('');
	});
});

describe('the address a message carries', () => {
	const projectURL = 'https://project.supabase.co';

	test('names the object itself, so a reader signs for it with their own session', () => {
		expect(attachmentAddress(projectURL, `${company}/shared/attachment/abc`)).toBe(
			`${projectURL}/storage/v1/object/asset/${company}/shared/attachment/abc`
		);
	});

	test('survives a project url written with a trailing slash', () => {
		expect(attachmentAddress(`${projectURL}/`, 'a/b')).toBe(attachmentAddress(projectURL, 'a/b'));
	});

});

describe('recognising a picture the company already keeps', () => {
	test('finds it under whatever extension its type gave it, by the hash it is asked for by', async () => {
		const lister: AssetLister & { looked: { path: string; search: string }[] } = {
			looked: [],
			list: async (path, options) => {
				lister.looked.push({ path, search: options.search });
				return { data: [{ name: '9f2c.jpg', metadata: { size: 1 } }], error: null };
			}
		};

		const kept = await sharedAssetKeptAs(lister, company, 'person-picture', '9f2c');

		expect(lister.looked).toEqual([{ path: `${company}/shared/person-picture`, search: '9f2c' }]);
		expect(kept).toBe(`${company}/shared/person-picture/9f2c.jpg`);
	});

	test('a longer hash that merely starts the same is not it', async () => {
		const lister: AssetLister = {
			list: async () => ({ data: [{ name: '9f2caaa.png', metadata: { size: 1 } }], error: null })
		};
		expect(await sharedAssetKeptAs(lister, company, 'person-picture', '9f2c')).toBeNull();
	});
});

describe('a refusal from the asset store', () => {
	test('says how big the file was and what it claimed to be', async () => {
		const uploader = {
			upload: async () => ({ error: { message: 'The object exceeded the maximum allowed size' } })
		} as unknown as Parameters<typeof keepSharedAsset>[0];

		const refusal = await keepSharedAsset(uploader, 'company-1', 'person-picture', new Uint8Array(213_000), 'image/png').catch(
			(error: Error) => error.message
		);

		expect(refusal).toContain('213000 bytes');
		expect(refusal).toContain('image/png');
		expect(refusal).toContain('exceeded the maximum allowed size');
	});

	test('names the missing content type rather than leaving a blank', async () => {
		const uploader = {
			upload: async () => ({ error: { message: 'refused' } })
		} as unknown as Parameters<typeof keepSharedAsset>[0];

		const refusal = await keepSharedAsset(uploader, 'company-1', 'person-picture', new Uint8Array(10), '').catch(
			(error: Error) => error.message
		);

		expect(refusal).toContain('no content type');
	});
});
