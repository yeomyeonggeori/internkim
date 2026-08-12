import { describe, expect, test } from 'bun:test';
import { digestOf, extensionOf, keepSharedAsset, sharedAssetPath, type AssetUploader } from './asset-store';

const company = '43000000-0000-0000-0000-0000000000a0';

function uploaderThat(refusal: string | null): AssetUploader & { written: string[] } {
	const written: string[] = [];
	return {
		written,
		upload: async (path) => {
			written.push(path);
			return { error: refusal ? { message: refusal } : null };
		},
		createSignedUrl: async (path) => ({
			data: { signedUrl: `https://project.test/object/sign/${path}?token=t` },
			error: null
		})
	};
}

describe('sharedAssetPath', () => {
	test('opens with the company, so the policy can read the owner off the path', async () => {
		const path = sharedAssetPath(company, 'link', await digestOf(new Uint8Array([1])), 'image/png');
		expect(path.startsWith(`${company}/shared/link/`)).toBe(true);
	});

	test('names the same bytes the same way, so the same picture is stored once', async () => {
		const bytes = new Uint8Array([1, 2, 3]);
		expect(sharedAssetPath(company, 'link', await digestOf(bytes), 'image/png')).toBe(
			sharedAssetPath(company, 'link', await digestOf(new Uint8Array([1, 2, 3])), 'image/png')
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

describe('keepSharedAsset', () => {
	test('returns the signed url the store gave it', async () => {
		const url = await keepSharedAsset(uploaderThat(null), company, 'link', new Uint8Array([1]), 'image/png');
		expect(url).toContain(`${company}/shared/link/`);
	});

	test('an asset already stored is the one we would have written, so it is not an error', async () => {
		const url = await keepSharedAsset(
			uploaderThat('The resource already exists'),
			company,
			'link',
			new Uint8Array([1]),
			'image/png'
		);
		expect(url).toContain('/shared/link/');
	});

	test('a refusal that is not about it already being there is raised', async () => {
		await expect(
			keepSharedAsset(uploaderThat('payload too large'), company, 'link', new Uint8Array([1]), 'image/png')
		).rejects.toThrow('payload too large');
	});
});
