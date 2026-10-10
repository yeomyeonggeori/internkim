import { describe, expect, test } from 'bun:test';
import {
	DataRoomRefused,
	neighbourID,
	previewSourceOf,
	type FileSigner,
	type ViewerDocument
} from '../../../src/lib/data-room/viewer';

function signerAnswering(answers: Record<string, string | number>): FileSigner & { asked: string[] } {
	const asked: string[] = [];
	const signer = async (derivedFileName?: string) => {
		const key = derivedFileName ?? 'original';
		asked.push(key);
		const answer = answers[key];
		if (typeof answer === 'number') throw new DataRoomRefused('refused', answer);
		if (answer === undefined) throw new Error(`unexpected request for ${key}`);
		return answer;
	};
	return Object.assign(signer, { asked });
}

describe('previewSourceOf', () => {
	test('draws the original when the reader may read it and the format is drawable', async () => {
		const signer = signerAnswering({ original: 'https://files/original' });
		expect(await previewSourceOf('report.pdf', signer, true)).toEqual({
			url: 'https://files/original',
			fileName: 'report.pdf',
			isTextPreview: false
		});
	});

	test('reads the extracted text for a view-only reader without asking for the original', async () => {
		const signer = signerAnswering({ 'content.txt': 'https://files/text' });
		expect(await previewSourceOf('report.pdf', signer, false)).toEqual({
			url: 'https://files/text',
			fileName: 'content.txt',
			isTextPreview: true
		});
		expect(signer.asked).toEqual(['content.txt']);
	});

	test('falls back to the extracted text when the original is refused', async () => {
		const signer = signerAnswering({ original: 403, 'content.txt': 'https://files/text' });
		expect(await previewSourceOf('cap-table.xlsx', signer, true)).toMatchObject({ isTextPreview: true });
	});

	test('reads the extracted text for a format it cannot draw', async () => {
		const signer = signerAnswering({ 'content.txt': 'https://files/text' });
		expect(await previewSourceOf('contract.docx', signer, true)).toMatchObject({ isTextPreview: true });
		expect(signer.asked).toEqual(['content.txt']);
	});

	test('answers nothing when neither file is readable', async () => {
		const signer = signerAnswering({ original: 400, 'content.txt': 404 });
		expect(await previewSourceOf('scan.png', signer, true)).toBeNull();
	});

	test('lets a failure that is not a refusal reach the caller', async () => {
		const signer = signerAnswering({ original: 502 });
		await expect(previewSourceOf('scan.png', signer, true)).rejects.toThrow('refused');
	});
});

describe('neighbourID', () => {
	const documents = ['a', 'b', 'c'].map(
		(id): ViewerDocument => ({ id, title: id, fileName: null, category: '', date: '', summary: '', details: [] })
	);

	test('steps to the document beside the open one', () => {
		expect(neighbourID(documents, 'b', -1)).toBe('a');
		expect(neighbourID(documents, 'b', 1)).toBe('c');
	});

	test('stops at either end and for a document no longer listed', () => {
		expect(neighbourID(documents, 'a', -1)).toBeUndefined();
		expect(neighbourID(documents, 'c', 1)).toBeUndefined();
		expect(neighbourID(documents, 'gone', 1)).toBeUndefined();
	});
});
