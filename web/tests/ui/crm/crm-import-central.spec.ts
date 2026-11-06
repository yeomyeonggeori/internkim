import { expect, test } from '@playwright/test';
import { signInToTheCRM } from './crm-central-test-utils';

test.use({ locale: 'ko-KR' });

const importedName = '샘플가져오기상사';

test('imports organizations from a CSV after a preview that saves nothing', async ({ page }) => {
	await signInToTheCRM(page);
	const panel = page.getByRole('tabpanel', { name: '관계처' });
	await panel.getByRole('button', { name: '더보기' }).click();
	await page.getByRole('menuitem', { name: '가져오기' }).click();

	const dialog = page.getByRole('dialog', { name: '관계처 가져오기' });
	await dialog.locator('input[type="file"]').setInputFiles({
		name: 'organizations.csv',
		mimeType: 'text/csv',
		buffer: Buffer.from(
			`이름,주소,비고2\n${importedName},"부산, ""해운대""\n2층",버릴 값\n예시 협력 기관,,\n,서울,\n`,
			'utf8'
		)
	});

	await expect(dialog.getByText('organizations.csv · 3줄 · 열 3개')).toBeVisible();
	await expect(dialog.getByRole('button', { name: '이름', exact: true })).toHaveText('이름 *');
	await expect(dialog.getByRole('button', { name: '비고2', exact: true })).toHaveText('가져오지 않음');
	await dialog.getByRole('button', { name: '다음' }).click();

	await expect(dialog.getByRole('cell', { name: importedName })).toBeVisible();
	await expect(dialog.getByText('이름 없음')).toBeVisible();
	await expect(dialog.getByText('이미 있음')).toBeVisible();
	await expect(panel.getByText(importedName)).toHaveCount(0);

	await dialog.getByRole('button', { name: '1건 저장' }).click();
	await expect(dialog).toBeHidden();
	await expect(panel.getByText(importedName)).toBeVisible();
});
