import { expect, test, type Page } from '@playwright/test';

async function mockLearningSettings(page: Page): Promise<void> {
	await page.route('**/admin/api/session', (route) => route.fulfill({ json: { email: 'operator@example.com', isAdmin: true, role: 'admin', isClaimed: true } }));
	await page.route('**/auth/session**', (route) => route.fulfill({ json: { authenticated: true, email: 'operator@example.com' } }));
	await page.route('**/admin/api/locale', (route) => route.fulfill({ json: { locale: 'ko' } }));
	await page.route('**/admin/api/workspace-settings', (route) => route.fulfill({ json: { timeZone: 'Asia/Seoul', language: 'ko' } }));
	await page.route('**/persona/api/agent', (route) => route.fulfill({ json: { schemaVersion: 1 } }));
	await page.route('**/persona/api/user', (route) => route.fulfill({ json: { schemaVersion: 1, callMe: '샘플 님' } }));
	await page.route('**/persona/api/identity', (route) => route.fulfill({ json: route.request().method() === 'POST' ? route.request().postDataJSON() : { schemaVersion: 1, names: ['김인턴', 'Intern Kim', '인턴킴'], handle: 'internkim', role: '업무 보조', introduction: '검증 가능한 도움을 제공합니다.' } }));
	await page.route('**/agent-learning/api/settings', (route) => route.fulfill({ json: { enabled: false, activeLimit: 20 } }));
	await page.route('**/agent-learning/api/skills?includeRetired=true', (route) => route.fulfill({ json: { settings: { enabled: false, activeLimit: 20 }, skills: [{ id: 'weekly-report', version: 2, audience: 'company', description: '주간 보고서를 정리할 때 적용합니다.', instruction: '자료를 모으고 확인한 뒤 요약합니다.', evidenceIDs: ['evidence:1', 'evidence:2'], reason: '반복된 보고서 작업에서 확인됨', verification: 'evidence-reviewed', status: 'active', protected: false, createdAt: '2026-08-01T00:00:00Z', updatedAt: '2026-08-20T00:00:00Z' }] } }));
	await page.route('**/agent-learning/api/skills/weekly-report?includeHistory=true', (route) => route.fulfill({ json: { skills: [{ id: 'weekly-report', version: 1, audience: 'company', description: '주간 보고서를 정리할 때 적용합니다.', instruction: '초기 절차를 확인합니다.', evidenceIDs: ['evidence:1'], reason: '초기 근거', verification: 'evidence-reviewed', status: 'retired', protected: false, createdAt: '2026-08-01T00:00:00Z', updatedAt: '2026-08-01T00:00:00Z' }, { id: 'weekly-report', version: 2, audience: 'company', description: '주간 보고서를 정리할 때 적용합니다.', instruction: '자료를 모으고 확인한 뒤 요약합니다.', evidenceIDs: ['evidence:1', 'evidence:2'], reason: '반복된 보고서 작업에서 확인됨', verification: 'evidence-reviewed', status: 'active', protected: false, createdAt: '2026-08-01T00:00:00Z', updatedAt: '2026-08-20T00:00:00Z' }] } }));
	await page.route('**/agent-learning/api/soul', (route) => route.request().method() === 'POST' ? route.fulfill({ json: { version: 3, document: route.request().postDataJSON().document, reason: '', createdAt: '2026-08-21T00:00:00Z', origin: 'administrator', author: 'person-sample' } }) : route.fulfill({ json: { version: 2, document: { schemaVersion: 1, values: ['사실을 확인합니다.'], boundaries: ['모르는 것을 단정하지 않습니다.'], workingStyle: ['작은 단계로 검증합니다.'], tone: { register: 'polite', traits: ['clear'] }, language: { default: 'ko' } }, reason: '업무 검증 원칙', createdAt: '2026-08-20T00:00:00Z', origin: 'review' } }));
	await page.route('**/agent-learning/api/soul/history', (route) => route.fulfill({ json: { history: [{ version: 1, document: { schemaVersion: 1, values: ['천천히 답합니다.'] }, reason: '초기 원칙', createdAt: '2026-08-01T00:00:00Z', origin: 'initial' }, { version: 2, document: { schemaVersion: 1, values: ['사실을 확인합니다.'] }, reason: '업무 검증 원칙', createdAt: '2026-08-20T00:00:00Z', origin: 'review' }] } }));
	await page.route('**/skills/api', (route) => route.fulfill({ json: { skills: [], unavailableSkills: [] } }));
	await page.route('**/api/v1/tools/**/invoke', (route) => route.fulfill({ json: { result: { categories: [], mutedConversationIDs: [], serverKey: '' } } }));
	await page.goto('/');
	const session = await page.evaluate(async () => {
		const response = await fetch('/auth/session');
		return { ok: response.ok, body: await response.json() };
	});
	expect(session.ok).toBe(true);
	expect(session.body).toMatchObject({ authenticated: true });
}

function cardTitled(page: Page, title: RegExp) {
	return page.locator('[data-slot="card"]').filter({ has: page.locator('[data-slot="card-title"]').filter({ hasText: title }) });
}

async function openAdminSettings(page: Page): Promise<void> {
	await page.goto('/settings/agent/');
	await page.evaluate(() => window.dispatchEvent(new Event('focus')));
}

async function dismissUnrelatedToasts(page: Page): Promise<void> {
	await page.locator('[data-sonner-toaster]').evaluateAll((elements) => elements.forEach((element) => element.remove()));
}

test('renders learned procedures and readable working principles on desktop', async ({ page }) => {
	await mockLearningSettings(page);
	await openAdminSettings(page);
	const learningCard = page.locator('[data-slot="card"]').filter({ has: page.locator('[data-slot="card-title"]').filter({ hasText: /^스스로 배우기$/ }) });
	await expect(learningCard.getByRole('switch', { name: '스스로 배우기' })).toBeVisible();
	await expect(learningCard.getByText('꺼져 있어도 배운 절차는 그대로 쓰고, 새로 배우거나 원칙을 고치지는 않습니다.')).toBeVisible();
	await expect(learningCard.getByText('주간 보고서를 정리할 때 적용합니다.')).toBeVisible();
	await learningCard.getByRole('button', { name: '내용과 이력' }).click();
	const historyPanel = learningCard.getByLabel('변경 이력');
	await historyPanel.locator('details summary').filter({ hasText: '버전 1' }).click();
	await expect(historyPanel.getByText('초기 절차를 확인합니다.')).toBeVisible();
	const principlesCard = page.locator('[data-slot="card"]').filter({ has: page.locator('[data-slot="card-title"]').filter({ hasText: /^일하는 원칙$/ }) });
	await expect(principlesCard.getByText('사실을 확인합니다.').first()).toBeVisible();
	await expect(principlesCard.getByText('해요체')).toBeVisible();
	await expect(principlesCard.getByText('버전 2 · 김인턴이 고침')).toBeVisible();
	await expect(principlesCard.getByText('+ 사실을 확인합니다.')).toBeVisible();
	await expect(principlesCard.getByText('− 천천히 답합니다.')).toBeVisible();
	await dismissUnrelatedToasts(page);
	await principlesCard.scrollIntoViewIfNeeded();
	await principlesCard.screenshot({ path: test.info().outputPath('desktop-principles.png') });
	await learningCard.scrollIntoViewIfNeeded();
	await learningCard.screenshot({ path: test.info().outputPath('desktop-learning.png') });
});

test('keeps the learning sections readable on mobile', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await mockLearningSettings(page);
	await openAdminSettings(page);
	const principlesCard = page.locator('[data-slot="card"]').filter({ has: page.locator('[data-slot="card-title"]').filter({ hasText: /^일하는 원칙$/ }) });
	const learningCard = page.locator('[data-slot="card"]').filter({ has: page.locator('[data-slot="card-title"]').filter({ hasText: /^스스로 배우기$/ }) });
	await expect(principlesCard).toBeVisible();
	await expect(learningCard).toBeVisible();
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
	await dismissUnrelatedToasts(page);
	await principlesCard.scrollIntoViewIfNeeded();
	await principlesCard.screenshot({ path: test.info().outputPath('mobile-principles.png') });
	await learningCard.scrollIntoViewIfNeeded();
	await learningCard.screenshot({ path: test.info().outputPath('mobile-learning.png') });
});

test('keeps the default nicknames and saves an added one with them', async ({ page }) => {
	await mockLearningSettings(page);
	await openAdminSettings(page);
	const identityCard = cardTitled(page, /^이름과 소개$/);
	await expect(identityCard.getByText('김인턴', { exact: true })).toBeVisible();
	await expect(identityCard.getByText('메신저에서는 @internkim')).toBeVisible();
	await expect(identityCard.getByRole('button', { name: 'Intern Kim 지우기' })).toHaveCount(0);
	await expect(identityCard.getByRole('button', { name: '저장' })).toBeDisabled();
	await identityCard.getByRole('button', { name: '별명 추가' }).click();
	await identityCard.getByRole('textbox', { name: '별명 추가' }).fill('샘플봇');
	await identityCard.getByRole('textbox', { name: '별명 추가' }).press('Enter');
	await expect(identityCard.getByRole('button', { name: '샘플봇 지우기' })).toBeVisible();
	const saved = page.waitForRequest((request) => request.url().includes('/persona/api/identity') && request.method() === 'POST');
	await identityCard.getByRole('button', { name: '저장' }).click();
	expect((await saved).postDataJSON().names).toEqual(['김인턴', 'Intern Kim', '인턴킴', '샘플봇']);
	await dismissUnrelatedToasts(page);
	await identityCard.scrollIntoViewIfNeeded();
	await identityCard.screenshot({ path: test.info().outputPath('desktop-identity.png') });
});

test('an administrator edits a principle and reverts the latest change', async ({ page }) => {
	await mockLearningSettings(page);
	await openAdminSettings(page);
	const principlesCard = cardTitled(page, /^일하는 원칙$/);
	await principlesCard.getByRole('button', { name: '편집' }).click();
	await principlesCard.getByRole('textbox', { name: '지키는 것' }).first().fill('확인한 사실만 말합니다.');
	await principlesCard.getByRole('button', { name: '줄 추가' }).nth(1).click();
	await principlesCard.getByRole('textbox', { name: '하지 않는 것' }).last().fill('샘플 금지 사항입니다.');
	await expect(principlesCard.getByText('확인했어요. 내일 다시 알려 드릴게요.')).toBeVisible();
	await principlesCard.screenshot({ path: test.info().outputPath('desktop-principles-editing.png') });
	const edited = page.waitForRequest((request) => request.url().includes('/agent-learning/api/soul') && request.method() === 'POST');
	await principlesCard.getByRole('button', { name: '저장' }).click();
	const editBody = (await edited).postDataJSON();
	expect(editBody.expectedVersion).toBe(2);
	expect(editBody.document.values).toEqual(['확인한 사실만 말합니다.']);
	expect(editBody.document.boundaries).toEqual(['모르는 것을 단정하지 않습니다.', '샘플 금지 사항입니다.']);
	await expect(principlesCard.getByRole('button', { name: '편집' })).toBeVisible();
	await principlesCard.getByRole('button', { name: '이 변경 되돌리기' }).click();
	const reverted = page.waitForRequest((request) => request.url().includes('/agent-learning/api/soul') && request.method() === 'POST');
	await principlesCard.getByRole('button', { name: '버전 1 내용으로 되돌리기' }).click();
	const revertBody = (await reverted).postDataJSON();
	expect(revertBody.document).toEqual({ schemaVersion: 1, values: ['천천히 답합니다.'] });
	expect(revertBody.reason).toBe('버전 1 내용으로 되돌림');
});
