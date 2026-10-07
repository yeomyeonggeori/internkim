import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import type { ChannelMessage } from '$lib/components/channel/channel-api';
import {
	conversationAsCSV,
	conversationAsPrintableHTML,
	conversationAsText,
	exportFilename,
	type ConversationDocument
} from '$lib/messenger/conversation-export';

const originalTimeZone = process.env.TZ;
beforeAll(() => {
	process.env.TZ = 'Asia/Seoul';
});
afterAll(() => {
	process.env.TZ = originalTimeZone;
});

function message(id: string, name: string, sentAt: string, text: string, filenames: string[] = []): ChannelMessage {
	return {
		id,
		sender: { id: name, name },
		text,
		sentAt,
		attachments: filenames.map((filename) => ({ kind: 'file', url: `https://example.com/${id}`, filename }))
	};
}

function documentOf(messages: ChannelMessage[]): ConversationDocument {
	return {
		title: '기획팀',
		savedAt: '2026-10-07T07:01:00Z',
		messages,
		labels: {
			savedAt: '저장한 시각',
			attachment: '파일',
			date: '날짜',
			time: '시간',
			sender: '보낸 사람',
			content: '내용',
			attachments: '첨부'
		}
	};
}

const conversation = documentOf([
	message('m1', '이샘플', '2026-10-06T07:01:00Z', '어제 회의록 올렸어요', ['회의록.pdf']),
	message('m2', '박예시', '2026-10-06T08:30:00Z', '확인했습니다'),
	message('m3', '이샘플', '2026-10-07T00:05:00Z', '가격은 "1,000원"이고\n다음 줄입니다')
]);

describe('the text export', () => {
	test('starts with the conversation and the time it was saved, then rules off each day', () => {
		const lines = conversationAsText(conversation).split('\n');
		expect(lines[0]).toBe('기획팀');
		expect(lines[1]).toMatch(/^저장한 시각: 2026년 10월 7일 .+ 오후 4:01$/);
		const rules = lines.filter((line) => line.startsWith('---'));
		expect(rules).toHaveLength(2);
		expect(rules[0]).toMatch(/^-+ 2026년 10월 6일 .+ -+$/);
		expect(rules[1]).toMatch(/^-+ 2026년 10월 7일 .+ -+$/);
	});

	test('writes each message as sender, time and text, with its files under it', () => {
		const text = conversationAsText(conversation);
		expect(text).toContain('[이샘플] [오후 4:01] 어제 회의록 올렸어요\n파일: 회의록.pdf\n[박예시] [오후 5:30] 확인했습니다');
		expect(text).toContain('[이샘플] [오전 9:05] 가격은 "1,000원"이고\n다음 줄입니다');
	});
});

describe('the CSV export', () => {
	test('opens with a byte order mark so Excel reads Korean, and a header row', () => {
		const csv = conversationAsCSV(conversation);
		expect(csv.startsWith('﻿날짜,시간,보낸 사람,내용,첨부\r\n')).toBe(true);
	});

	test('keeps one row per message and quotes commas, quotes and line breaks', () => {
		const rows = conversationAsCSV(conversation).slice(1).split('\r\n');
		expect(rows[1]).toBe('2026-10-06,오후 4:01,이샘플,어제 회의록 올렸어요,회의록.pdf');
		expect(rows[3]).toBe('2026-10-07,오전 9:05,이샘플,"가격은 ""1,000원""이고\n다음 줄입니다",');
		expect(rows).toHaveLength(5);
		expect(rows[4]).toBe('');
	});

	test('keeps a message that starts like a formula from running as one', () => {
		const csv = conversationAsCSV(documentOf([message('m1', '최견본', '2026-10-06T07:01:00Z', '=SUM(A1:A2)')]));
		expect(csv).toContain(",'=SUM(A1:A2),");
	});
});

describe('the printable export', () => {
	test('escapes what people wrote instead of rendering it', () => {
		const html = conversationAsPrintableHTML(
			documentOf([message('m1', '최견본', '2026-10-06T07:01:00Z', '<img src=x onerror=alert(1)>')])
		);
		expect(html).toContain('&lt;img src=x onerror=alert(1)&gt;');
		expect(html).not.toContain('<img');
	});
});

test('a file name keeps the conversation name and drops characters a file system refuses', () => {
	expect(exportFilename('기획/운영: 1팀', '2026-10-07T07:01:00Z', 'csv')).toBe('기획_운영_ 1팀-2026-10-07.csv');
});
