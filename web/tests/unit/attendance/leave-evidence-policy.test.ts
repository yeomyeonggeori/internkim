import { describe, expect, test } from 'bun:test';
import {
	leaveEvidenceMaximumFileSizeBytes,
	validateLeaveEvidenceFiles
} from '../../../src/routes/attendance/leave/leave-evidence-policy';

describe('leave evidence policy', () => {
	test('accepts supported image and PDF evidence within the size limit', () => {
		const image = new File(['image'], 'evidence.JPG', { type: 'image/jpeg' });
		const pdf = new File(
			[new Uint8Array(leaveEvidenceMaximumFileSizeBytes)],
			'evidence.pdf',
			{ type: 'application/pdf' }
		);

		expect(validateLeaveEvidenceFiles(0, [image, pdf])).toEqual({
			acceptedFiles: [image, pdf]
		});
	});

	test('rejects unsupported and oversized evidence without accepting it', () => {
		const unsupported = new File(['notes'], 'notes.txt', { type: 'text/plain' });
		const oversized = new File(
			[new Uint8Array(leaveEvidenceMaximumFileSizeBytes + 1)],
			'evidence.pdf',
			{ type: 'application/pdf' }
		);

		expect(validateLeaveEvidenceFiles(0, [unsupported])).toEqual({
			acceptedFiles: [],
			violation: 'invalidType'
		});
		expect(validateLeaveEvidenceFiles(0, [oversized])).toEqual({
			acceptedFiles: [],
			violation: 'tooLarge'
		});
	});

	test('keeps the total evidence count at five', () => {
		const sixthFile = new File(['evidence'], 'sixth.pdf', { type: 'application/pdf' });

		expect(validateLeaveEvidenceFiles(5, [sixthFile])).toEqual({
			acceptedFiles: [],
			violation: 'tooMany'
		});
	});
});
