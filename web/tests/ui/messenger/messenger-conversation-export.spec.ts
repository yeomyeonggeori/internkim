import { readFile } from 'node:fs/promises';
import { expect, test, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const reader = { id: 'person-reader', email: 'reader@example.com' };
const pageSize = 10;
const pageCount = 3;

const conversations = [
	{
		id: 'channel-plan',
		name: '기획팀',
		kind: 'group' as const,
		myRole: 'member',
		members: [
			{ externalID: 'sample-lee', name: '이샘플', role: 'owner' },
			{ externalID: 'sample-park', name: '박예시', role: 'member' }
		]
	},
	{ id: 'dm-park', name: '박예시', kind: 'dm' as const, myRole: 'member' }
];

const history = Array.from({ length: pageSize * pageCount }, (_, index) => ({
	id: `message-${String(index + 1).padStart(2, '0')}`,
	sender: { id: index % 2 === 0 ? 'sample-lee' : 'sample-park', name: index % 2 === 0 ? '이샘플' : '박예시' },
	text: `기록 ${index + 1}번, "따옴표"`,
	sentAt: new Date(Date.UTC(2026, 9, 1 + Math.floor(index / pageSize), 1, index)).toISOString()
}));

async function serveHistoryInPages(page: Page): Promise<string[]> {
	const cursorsAsked: string[] = [];
	await page.route('**/agent/api/dm**', async (route) => {
		const before = new URL(route.request().url()).searchParams.get('before') ?? '';
		cursorsAsked.push(before);
		const end = before ? history.findIndex((message) => message.id === before) : history.length;
		const start = Math.max(0, end - pageSize);
		await route.fulfill({
			json: {
				conversationID: conversations[0].id,
				currentUserId: reader.id,
				messages: history.slice(start, end),
				hasMoreBefore: start > 0,
				historyCursor: history[start]?.id ?? ''
			}
		});
	});
	return cursorsAsked;
}

test.use({ viewport: { width: 1280, height: 800 }, timezoneId: 'Asia/Seoul', locale: 'ko-KR' });

test('a channel exports every page of its history as text from the channel details', async ({ page }) => {
	await mockDeviceMessenger(page, reader, conversations, []);
	const cursorsAsked = await serveHistoryInPages(page);
	await page.goto('/messenger?channel=channel-plan');

	await page.getByRole('button', { name: '채널 정보' }).click();
	const details = page.getByRole('dialog', { name: '채널 정보' });
	const rowHeights = await details.locator('[data-slot="item"]').evaluateAll((rows) =>
		rows.map((row) => Math.round(row.getBoundingClientRect().height))
	);
	expect(new Set(rowHeights).size).toBe(1);
	await page.screenshot({ path: test.info().outputPath('channel-details.png') });

	await details.getByRole('button', { name: '대화 내보내기' }).click();

	const dialog = page.getByRole('dialog', { name: '기획팀 대화 내보내기' });
	await expect(dialog.getByText('[이샘플] [오전 10:00] 기록 1번, "따옴표"')).toBeVisible();
	await expect(dialog.getByText('… 외 22개')).toBeVisible();
	await page.screenshot({ path: test.info().outputPath('export-text.png') });

	const download = page.waitForEvent('download');
	await dialog.getByRole('button', { name: '내보내기', exact: true }).click();
	const file = await download;
	expect(file.suggestedFilename()).toMatch(/^기획팀-\d{4}-\d{2}-\d{2}\.txt$/);

	const text = await readFile(await file.path(), 'utf8');
	for (const message of history) expect(text).toContain(`] ${message.text}`);
	expect(text.match(/^-+ .+ -+$/gm)).toHaveLength(pageCount);
	expect(cursorsAsked).toContain('message-11');
	expect(cursorsAsked).toContain('message-21');
});

test('a direct message exports as CSV from the conversation menu', async ({ page }) => {
	await mockDeviceMessenger(page, reader, conversations, []);
	await serveHistoryInPages(page);
	await page.goto('/messenger?channel=channel-plan');

	await page.locator('[data-sidebar="menu-item"]').filter({ hasText: '박예시' }).click({ button: 'right' });
	await page.getByRole('menuitem', { name: '대화 내보내기' }).click();

	const dialog = page.getByRole('dialog', { name: '박예시 대화 내보내기' });
	await dialog.getByRole('radio', { name: /CSV/ }).click();
	await expect(dialog.getByText('날짜,시간,보낸 사람,내용,첨부')).toBeVisible();
	await dialog.getByRole('radio', { name: /PDF/ }).click();
	await expect(dialog.frameLocator('iframe').getByText('기록 1번, "따옴표"')).toBeVisible();
	await page.screenshot({ path: test.info().outputPath('export-pdf.png') });
	await dialog.getByRole('radio', { name: /CSV/ }).click();

	const download = page.waitForEvent('download');
	await dialog.getByRole('button', { name: '내보내기', exact: true }).click();
	const file = await download;
	expect(file.suggestedFilename()).toMatch(/^박예시-\d{4}-\d{2}-\d{2}\.csv$/);

	const bytes = await readFile(await file.path());
	expect([...bytes.subarray(0, 3)]).toEqual([0xef, 0xbb, 0xbf]);
	const rows = bytes.toString('utf8').trim().split('\r\n');
	expect(rows).toHaveLength(history.length + 1);
	expect(rows[1]).toBe('2026-10-01,오전 10:00,이샘플,"기록 1번, ""따옴표""",');
});
