import { expect, test, type Page } from '@playwright/test';
import { mockDeviceMessenger } from './messenger-device-mock';

const channelID = 'channel-owners-channel';
const reader = { id: 'person-reader', name: '이샘플', email: 'reader@example.com' };

async function openDetails(page: Page, myRole: 'owner' | 'member') {
	await page.setViewportSize({ width: 390, height: 800 });
	await mockDeviceMessenger(page, reader, [{
		id: channelID,
		name: '주인 확인 채널',
		kind: 'group',
		myRole,
		members: [
			{ externalID: reader.id, name: reader.name, role: myRole },
			{ externalID: 'person-owner', name: '박예시', role: 'owner' },
			{ externalID: 'person-member', name: '최견본', role: 'member' }
		]
	}], []);
	await page.goto(`/messenger?channel=${channelID}`);
	await page.getByRole('button', { name: '채널 정보' }).click();
	const details = page.getByRole('dialog', { name: '채널 정보' });
	await expect(details).toBeVisible();
	return details;
}

test('channel owners fold to their avatars and unfold into a list with the owner actions', async ({ page }) => {
	const details = await openDetails(page, 'owner');
	const toggle = details.getByRole('button', { name: /^채널 주인/ });
	await expect(toggle).toHaveAttribute('aria-expanded', 'false');
	await expect(toggle).not.toContainText('명');

	await toggle.click();
	const owners = details.getByRole('list', { name: '채널 주인' });
	await expect(owners.getByText('이샘플', { exact: true })).toBeVisible();
	await expect(owners.getByText('박예시', { exact: true })).toBeVisible();
	await expect(owners.getByText('최견본', { exact: true })).toHaveCount(0);

	await details.getByRole('button', { name: '주인 넘기기', exact: true }).click();
	await page.getByRole('option', { name: '최견본' }).click();
	await details.getByRole('button', { name: '넘기기', exact: true }).click();
	const confirm = page.getByRole('alertdialog', { name: '최견본님에게 주인을 넘길까요?' });
	await expect(confirm).toBeVisible();
	await confirm.getByRole('button', { name: '취소' }).click();
	await expect(confirm).toBeHidden();
	await expect(details).toBeVisible();
});

test('a member who is not an owner sees the owners without the owner actions', async ({ page }) => {
	const details = await openDetails(page, 'member');
	await details.getByRole('button', { name: /^채널 주인/ }).click();
	await expect(details.getByRole('list', { name: '채널 주인' }).getByText('박예시', { exact: true })).toBeVisible();
	await expect(details.getByRole('button', { name: '주인 추가', exact: true })).toHaveCount(0);
	await expect(details.getByRole('button', { name: '주인 넘기기', exact: true })).toHaveCount(0);
});
