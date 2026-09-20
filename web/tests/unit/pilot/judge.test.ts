import { describe, expect, test } from 'bun:test';
import type { DeliveredFile } from '../../pilot/arms/arm';
import {
	deliveredFileFinding,
	judge,
	replyMentionsFinding,
	waitingForAnswerFinding,
	type JudgedOutcome,
} from '../../pilot/record';

function deliveredFile(overrides: Partial<DeliveredFile>): DeliveredFile {
	return {
		filename: 'pipeline.xlsx',
		contentType: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
		sizeBytes: 12000,
		devicePath: '/workspace/private/people/person/documents/pipeline.xlsx',
		isZipContainer: true,
		...overrides,
	};
}

function outcome(overrides: Partial<JudgedOutcome>): JudgedOutcome {
	return { status: 'completed', reply: '', deliveredFiles: [], ...overrides };
}

describe('the delivered file assertion', () => {
	test('accepts a file whose name, size and container all hold', () => {
		const finding = deliveredFileFinding({ deliveredFile: { filenameEndsWith: '.xlsx', minBytes: 2000 } }, [deliveredFile({})]);
		expect(finding.mismatches).toEqual([]);
		expect(finding.notes).toEqual([]);
	});

	test('refuses a run that delivered nothing with that name', () => {
		const finding = deliveredFileFinding({ deliveredFile: { filenameEndsWith: '.docx' } }, [deliveredFile({})]);
		expect(finding.rowCount).toBe(0);
		expect(finding.mismatches).toEqual(['no delivered file ends with .docx']);
	});

	test('refuses a file under the minimum size', () => {
		const finding = deliveredFileFinding({ deliveredFile: { filenameEndsWith: '.xlsx', minBytes: 20000 } }, [deliveredFile({ sizeBytes: 900 })]);
		expect(finding.mismatches).toEqual(['sizeBytes: expected at least 20000, found 900']);
	});

	test('refuses an office file that is not a readable zip container', () => {
		const finding = deliveredFileFinding({ deliveredFile: { filenameEndsWith: '.docx' } }, [
			deliveredFile({ filename: 'brief.docx', isZipContainer: false }),
		]);
		expect(finding.mismatches).toEqual(['brief.docx is not a readable zip container']);
	});

	test('accepts the metadata alone when the arm could not read the bytes, and says so', () => {
		const finding = deliveredFileFinding({ deliveredFile: { filenameEndsWith: '.pptx', minBytes: 1000 } }, [
			deliveredFile({ filename: 'deck.pptx', isZipContainer: null }),
		]);
		expect(finding.mismatches).toEqual([]);
		expect(finding.notes).toEqual(['deck.pptx: the arm could not read the bytes, so only the attachment metadata was checked']);
	});

	test('leaves the zip container question alone for a file that is not one', () => {
		const finding = deliveredFileFinding({ deliveredFile: { filenameEndsWith: '.pdf' } }, [
			deliveredFile({ filename: 'status.pdf', contentType: 'application/pdf', isZipContainer: false }),
		]);
		expect(finding.mismatches).toEqual([]);
		expect(finding.notes).toEqual([]);
	});

	test('refuses a content type the run did not deliver', () => {
		const finding = deliveredFileFinding({ deliveredFile: { filenameEndsWith: '.xlsx', contentType: 'application/vnd.ms-excel' } }, [
			deliveredFile({ contentType: 'application/octet-stream' }),
		]);
		expect(finding.mismatches).toEqual(['contentType: expected application/vnd.ms-excel, found application/octet-stream']);
	});

	test('accepts the one good file among several with the same suffix', () => {
		const finding = deliveredFileFinding({ deliveredFile: { filenameEndsWith: '.xlsx', minBytes: 2000 } }, [
			deliveredFile({ filename: 'draft.xlsx', sizeBytes: 120 }),
			deliveredFile({ filename: 'pipeline.xlsx', sizeBytes: 9000 }),
		]);
		expect(finding.rowCount).toBe(2);
		expect(finding.mismatches).toEqual([]);
	});
});

describe('the reply mentions assertion', () => {
	test('accepts a reply carrying every value the record holds', () => {
		const finding = replyMentionsFinding({ replyMentions: ['정견본', '재무 이사'] }, '견본 물산 창구는 재무 이사 정견본입니다.');
		expect(finding.mismatches).toEqual([]);
		expect(finding.rowCount).toBe(2);
	});

	test('names the value the reply left out', () => {
		const finding = replyMentionsFinding({ replyMentions: ['정견본', '재무 이사'] }, '견본 물산 창구는 정견본입니다.');
		expect(finding.mismatches).toEqual(['the reply does not mention "재무 이사"']);
	});
});

describe('the waiting for an answer assertion', () => {
	test('accepts a run paused on a question about the withheld fact', () => {
		const finding = waitingForAnswerFinding(
			{ waitingForAnswer: { mentions: ['담당자'] } },
			outcome({ status: 'waiting_user_input', reply: '두 곳 담당자를 누구로 바꿀까요?' }),
		);
		expect(finding.mismatches).toEqual([]);
		expect(finding.notes).toEqual([]);
	});

	test('accepts a harness without a paused state when its final reply asks, and says so', () => {
		const finding = waitingForAnswerFinding(
			{ waitingForAnswer: { mentions: ['담당자'] } },
			outcome({ status: 'completed', reply: '새 담당자를 알려주세요.' }),
		);
		expect(finding.mismatches).toEqual([]);
		expect(finding.notes).toEqual(['the arm has no paused state, so the question was read from the final reply']);
	});

	test('refuses a run that paused without naming the withheld fact', () => {
		const finding = waitingForAnswerFinding(
			{ waitingForAnswer: { mentions: ['담당자'] } },
			outcome({ status: 'waiting_user_input', reply: '언제까지 해야 하나요?' }),
		);
		expect(finding.mismatches).toEqual(['the question mentions none of ["담당자"]']);
	});

	test('refuses a run that failed instead of asking', () => {
		const finding = waitingForAnswerFinding(
			{ waitingForAnswer: { mentions: ['담당자'] } },
			outcome({ status: 'failed', reply: '담당자를 찾지 못했습니다.' }),
		);
		expect(finding.mismatches).toEqual(['expected the run to stop and ask, found failed']);
	});
});

describe('the overall pilot judgement', () => {
	const record = { apiURL: '', secretKey: '' };
	const fileAssertion = { deliveredFile: { filenameEndsWith: '.docx', minBytes: 1000 } };
	const deliveredDocument = deliveredFile({ filename: 'brief.docx' });

	for (const status of ['failed', 'timed_out'] as const) {
		test(`does not pass a ${status} run with valid file metadata`, async () => {
			const judgement = await judge(record, [fileAssertion], outcome({ status, deliveredFiles: [deliveredDocument] }));

			expect(judgement.passed).toBe(false);
			expect(judgement.findings.find((finding) => finding.subject === 'deliveredFile')?.mismatches).toEqual([]);
			expect(judgement.findings.find((finding) => finding.subject === 'runStatus')?.mismatches).toEqual([
				`the run ended with status ${status}`,
			]);
		});
	}

	test('passes a completed run when its evidence assertions pass', async () => {
		const judgement = await judge(record, [fileAssertion], outcome({ deliveredFiles: [deliveredDocument] }));

		expect(judgement.passed).toBe(true);
	});

	test('passes a waiting run when its file and question assertions pass', async () => {
		const judgement = await judge(
			record,
			[fileAssertion, { waitingForAnswer: { mentions: ['담당자'] } }],
			outcome({ status: 'waiting_user_input', reply: '담당자를 누구로 바꿀까요?', deliveredFiles: [deliveredDocument] }),
		);

		expect(judgement.passed).toBe(true);
		expect(judgement.findings.find((finding) => finding.subject === 'waitingForAnswer')?.mismatches).toEqual([]);
	});
});
