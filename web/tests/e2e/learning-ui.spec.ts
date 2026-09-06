import { expect, test, type Page } from '@playwright/test';

async function mockLearningSettings(page: Page): Promise<void> {
	await page.route('**/admin/api/session', (route) => route.fulfill({ json: { email: 'operator@example.com', isAdmin: true, role: 'admin', isClaimed: true } }));
	await page.route('**/auth/session**', (route) => route.fulfill({ json: { authenticated: true, email: 'operator@example.com' } }));
	await page.route('**/admin/api/locale', (route) => route.fulfill({ json: { locale: 'ko' } }));
	await page.route('**/admin/api/workspace-settings', (route) => route.fulfill({ json: { timeZone: 'Asia/Seoul', language: 'ko' } }));
	await page.route('**/persona/api/agent', (route) => route.fulfill({ json: { schemaVersion: 1 } }));
	await page.route('**/persona/api/user', (route) => route.fulfill({ json: { schemaVersion: 1, callMe: '샘플 님' } }));
	await page.route('**/persona/api/identity', (route) => route.fulfill({ json: { schemaVersion: 1, names: ['샘플 에이전트'], handle: 'sample-agent', role: '업무 보조', creature: 'assistant', emoji: '🌱', introduction: '검증 가능한 도움을 제공합니다.' } }));
	await page.route('**/agent-learning/api/settings', (route) => route.fulfill({ json: { enabled: false, activeLimit: 20 } }));
	await page.route('**/agent-learning/api/skills?includeRetired=true', (route) => route.fulfill({ json: { settings: { enabled: false, activeLimit: 20 }, skills: [{ id: 'weekly-report', version: 2, audience: 'company', description: '주간 보고서를 정리할 때 적용합니다.', instruction: '자료를 모으고 확인한 뒤 요약합니다.', evidenceIDs: ['evidence:1', 'evidence:2'], reason: '반복된 보고서 작업에서 확인됨', verification: 'evidence-reviewed', status: 'active', protected: false, createdAt: '2026-08-01T00:00:00Z', updatedAt: '2026-08-20T00:00:00Z' }] } }));
	await page.route('**/agent-learning/api/soul', (route) => route.fulfill({ json: { version: 2, document: { schemaVersion: 1, values: ['사실을 확인합니다.'], boundaries: ['모르는 것을 단정하지 않습니다.'], workingStyle: ['작은 단계로 검증합니다.'], tone: { register: 'polite', traits: ['clear'] }, language: { default: 'ko' } }, reason: '업무 검증 원칙', createdAt: '2026-08-20T00:00:00Z', origin: 'review' } }));
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

async function openAdminSettings(page: Page): Promise<void> {
	await page.goto('/settings/');
	await page.evaluate(() => window.dispatchEvent(new Event('focus')));
	await page.getByRole('tab', { name: '관리자' }).click();
}

async function dismissUnrelatedToasts(page: Page): Promise<void> {
	await page.locator('[data-sonner-toaster]').evaluateAll((elements) => elements.forEach((element) => element.remove()));
}

test('renders learned procedures and readable working principles on desktop', async ({ page }) => {
	await mockLearningSettings(page);
	await openAdminSettings(page);
	await expect(page.getByText('배운 절차')).toBeVisible();
	await expect(page.getByText('주간 보고서를 정리할 때 적용합니다.')).toBeVisible();
	await expect(page.getByText('현재 원칙')).toBeVisible();
	await expect(page.getByText('사실을 확인합니다.')).toBeVisible();
	await dismissUnrelatedToasts(page);
	const learningArea = page.locator('[data-slot="card"]').filter({ hasText: '현재 원칙' }).first();
	await learningArea.scrollIntoViewIfNeeded();
	await learningArea.screenshot({ path: '.artifacts/learning-ui/desktop-learning-area.png' });
});

test('keeps the learning sections readable on mobile', async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await mockLearningSettings(page);
	await openAdminSettings(page);
	await expect(page.getByText('배운 절차')).toBeVisible();
	await expect(page.getByText('현재 원칙')).toBeVisible();
	await dismissUnrelatedToasts(page);
	const learningArea = page.locator('[data-slot="card"]').filter({ hasText: '현재 원칙' }).first();
	await learningArea.scrollIntoViewIfNeeded();
	await learningArea.screenshot({ path: '.artifacts/learning-ui/mobile-learning-area.png' });
});
