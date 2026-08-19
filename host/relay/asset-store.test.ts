import { describe, expect, test } from 'bun:test';
import {
	attachmentAddress,
	attachmentAlreadyKept,
	digestOf,
	extensionOf,
	keepMessageAttachment,
	sharedAssetPath,
	type AssetLister,
	type AssetUploader
} from './asset-store';

const company = '43000000-0000-0000-0000-0000000000a0';

function listerHolding(names: string[]): AssetLister & { looked: { path: string; search: string }[] } {
	const looked: { path: string; search: string }[] = [];
	return {
		looked,
		list: async (path, options) => {
			looked.push({ path, search: options.search });
			return {
				data: names
					.filter((name) => name === options.search)
					.map((name) => ({ name, metadata: { size: 91_000_000 } })),
				error: null
			};
		}
	};
}

function uploaderThat(refusal: string | null): AssetUploader & { written: string[] } {
	const written: string[] = [];
	return {
		written,
		upload: async (path) => {
			written.push(path);
			return { error: refusal ? { message: refusal } : null };
		}
	};
}

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

describe('keepMessageAttachment', () => {
	test('keeps a file the relay would refuse, under the company that sent it', async () => {
		const uploader = uploaderThat(null);
		const kept = await keepMessageAttachment(uploader, company, new Uint8Array([1, 2]), 'text/html');
		expect(kept.path.startsWith(`${company}/shared/attachment/`)).toBe(true);
		expect(kept.path).toContain(kept.digest);
		expect(uploader.written).toEqual([kept.path]);
	});

	test('gives back the path rather than a signed url, so nothing long-lived is written down', async () => {
		const kept = await keepMessageAttachment(uploaderThat(null), company, new Uint8Array([1]), 'audio/mpeg');
		expect(kept.path).not.toContain('token');
		expect(kept.path).not.toContain('http');
	});

	test('the same file sent twice is stored once', async () => {
		const first = await keepMessageAttachment(uploaderThat(null), company, new Uint8Array([7]), 'text/html');
		const again = await keepMessageAttachment(
			uploaderThat('The resource already exists'),
			company,
			new Uint8Array([7]),
			'text/html'
		);
		expect(again.path).toBe(first.path);
	});

	test('a refusal that is not about it already being there is raised', async () => {
		await expect(
			keepMessageAttachment(uploaderThat('payload too large'), company, new Uint8Array([1]), 'text/html')
		).rejects.toThrow('payload too large');
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

describe('recognising a file the company already keeps', () => {
	test('looks for the object those exact bytes would be, and reports its size', async () => {
		const lister = listerHolding(['9f2c']);

		const kept = await attachmentAlreadyKept(lister, company, '9f2c', 'application/pdf');

		expect(lister.looked).toEqual([{ path: `${company}/shared/attachment`, search: '9f2c' }]);
		expect(kept).toEqual({ path: `${company}/shared/attachment/9f2c`, sizeBytes: 91_000_000 });
	});

	test('a picture is looked for under the extension it was kept with', async () => {
		const lister = listerHolding(['9f2c.png']);

		const kept = await attachmentAlreadyKept(lister, company, '9f2c', 'image/png');

		expect(kept?.path).toBe(`${company}/shared/attachment/9f2c.png`);
	});

	test('bytes the company has never kept are not there', async () => {
		expect(await attachmentAlreadyKept(listerHolding([]), company, '9f2c', 'application/pdf')).toBeNull();
	});

	test('a listing that answers with a longer name is not a match', async () => {
		const lister: AssetLister = {
			list: async () => ({ data: [{ name: '9f2caaa', metadata: { size: 1 } }], error: null })
		};

		expect(await attachmentAlreadyKept(lister, company, '9f2c', 'application/pdf')).toBeNull();
	});
});

describe('a refusal from the asset store', () => {
	test('says how big the file was and what it claimed to be', async () => {
		const uploader = {
			upload: async () => ({ error: { message: 'The object exceeded the maximum allowed size' } })
		} as unknown as Parameters<typeof keepMessageAttachment>[0];

		const refusal = await keepMessageAttachment(uploader, 'company-1', new Uint8Array(213_000), 'image/png').catch(
			(error: Error) => error.message
		);

		expect(refusal).toContain('213000 bytes');
		expect(refusal).toContain('image/png');
		expect(refusal).toContain('exceeded the maximum allowed size');
	});

	test('names the missing content type rather than leaving a blank', async () => {
		const uploader = {
			upload: async () => ({ error: { message: 'refused' } })
		} as unknown as Parameters<typeof keepMessageAttachment>[0];

		const refusal = await keepMessageAttachment(uploader, 'company-1', new Uint8Array(10), '').catch(
			(error: Error) => error.message
		);

		expect(refusal).toContain('no content type');
	});
});
