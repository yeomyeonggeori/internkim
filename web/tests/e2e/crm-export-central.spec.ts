import { readFile } from 'node:fs/promises';
import { expect, test } from '@playwright/test';
import { signInToTheCRM } from './crm-central-test-utils';

test.use({ locale: 'ko-KR' });

test('exports only the contacts of the organization chosen in the export dialog', async ({ page }) => {
	await signInToTheCRM(page);
	await page.getByRole('tab', { name: '연락처', exact: true }).click();
	await page.getByRole('tabpanel', { name: '연락처' }).getByRole('button', { name: '더보기' }).click();
	await page.getByRole('menuitem', { name: '내보내기' }).click();

	const dialog = page.getByRole('dialog', { name: '내보내기' });
	await expect(dialog.getByRole('tab', { name: '연락처' })).toHaveAttribute('aria-selected', 'true');
	await expect(dialog.getByRole('checkbox', { name: '최견본 선택' })).toBeChecked();
	await expect(dialog.getByRole('checkbox', { name: '박견본 선택' })).toBeChecked();

	await dialog.getByRole('button', { name: '관계처', exact: true }).click();
	await page.getByRole('option', { name: '예시 협력 기관' }).click();
	await expect(dialog.getByRole('checkbox', { name: '최견본 선택' })).toBeChecked();
	await expect(dialog.getByRole('checkbox', { name: '박견본 선택' })).not.toBeChecked();

	const downloading = page.waitForEvent('download');
	await dialog.getByRole('button', { name: /CSV 내려받기/ }).click();
	const download = await downloading;
	expect(download.suggestedFilename()).toMatch(/^CRM-연락처-\d{4}-\d{2}-\d{2}\.csv$/);
	const content = await readFile(await download.path(), 'utf8');
	expect(content.startsWith('﻿')).toBe(true);
	expect(content).toContain('최견본,예시 협력 기관,프로그램 매니저,협력팀,contact.one@example.com');
	expect(content).not.toContain('박견본');
	await expect(dialog).toBeHidden();
});
