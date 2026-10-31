import { expect, test, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';
import { everyScreen } from '../support/screen-widths';

const channelID = 'channel-members-channel';
const reader = { id: 'person-reader', name: '이샘플', email: 'reader@example.com' };
const memberNames = ['박예시', '최견본', '김샘플', '정예시', '한견본', '오샘플', '윤예시'];

async function openChannel(page: Page, width: number) {
	await page.setViewportSize({ width, height: 800 });
	await mockDeviceMessenger(page, reader, [{
		id: channelID,
		name: '멤버 확인 채널',
		kind: 'group',
		myRole: 'member',
		members: memberNames.map((name, index) => ({ externalID: `person-${index}`, name, role: index === 0 ? 'owner' : 'member' }))
	}], []);
	await page.goto(`/messenger?channel=${channelID}`);
	await expect(page.getByRole('combobox', { name: '메시지를 입력하세요' })).toBeVisible();
}

for (const width of everyScreen) {
	test(`the member count opens channel details with the member list unfolded at ${width}px`, async ({ page }) => {
		await openChannel(page, width);
		await page.getByRole('button', { name: `멤버 ${memberNames.length}명`, exact: true }).click();

		const details = page.getByRole('dialog', { name: '채널 정보' });
		await expect(details).toBeVisible();
		await expect.poll(async () => {
			const box = await details.boundingBox();
			return box ? box.x + box.width : Number.POSITIVE_INFINITY;
		}).toBeLessThanOrEqual(width);
		expect((await details.boundingBox())!.x).toBeGreaterThanOrEqual(0);

		const list = details.getByRole('list', { name: '채널 멤버' });
		await expect(list.getByRole('listitem')).toHaveCount(5);
		await expect(list.getByText('박예시', { exact: true })).toBeVisible();
		await details.getByRole('button', { name: '모두 보기 (2명 더)' }).click();
		await expect(list.getByRole('listitem')).toHaveCount(memberNames.length);

		const toggle = details.getByRole('button', { name: /^채널 멤버/ });
		await expect(toggle).toHaveAttribute('aria-expanded', 'true');
		await toggle.click();
		await expect(toggle).toHaveAttribute('aria-expanded', 'false');
		await expect(list).toBeHidden();
	});
}

test('channel details opened from its own button keep the member list folded', async ({ page }) => {
	await openChannel(page, 390);
	await page.getByRole('button', { name: '채널 정보' }).click();
	const details = page.getByRole('dialog', { name: '채널 정보' });
	await expect(details.getByRole('button', { name: /^채널 멤버/ })).toHaveAttribute('aria-expanded', 'false');
	await expect(details.getByRole('list', { name: '채널 멤버' })).toBeHidden();
});
