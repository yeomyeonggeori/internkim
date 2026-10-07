import { expect, test, type Page } from '@playwright/test';
import { mkdir, writeFile } from 'node:fs/promises';
import { AttendanceLoadingFixture, LoadingResponseGate, loadingFixtureTime } from './attendance-loading-fixture';
import { calendarViewStorageKey } from '../../src/routes/calendar/calendar-storage-keys';

const before = process.env.LOADING_EVIDENCE_PHASE === 'before';
const directory = process.env.LOADING_EVIDENCE_DIR;
test.use({ locale: 'ko-KR', colorScheme: 'light', contextOptions: { reducedMotion: 'reduce' } });

async function prepare(page: Page, view: string, empty = false) {
	const fixture = new AttendanceLoadingFixture();
	await fixture.install(page);
	await page.addInitScript(({ key, view }) => localStorage.setItem(key, view), { key: calendarViewStorageKey, view });
	const response = { gate: new LoadingResponseGate(), empty, status: 200, startsAt: '2026-10-06T01:00:00.000Z', endsAt: '2026-10-06T02:00:00.000Z', participantID: '' };
	await page.route('**/api/v1/tools/event_list/invoke', async route => {
		await response.gate.wait();
		if (response.status !== 200) {
			await route.fulfill({ status: response.status, json: { error: response.status === 403 ? '일정 조회 권한이 없습니다' : '일정을 불러오지 못했습니다' } });
			return;
		}
		const participants = response.participantID ? [{ personID: response.participantID, name: '박예시2', email: 'sample1@example.com' }] : [];
		const events = response.empty ? [] : [{ eventID: 'sample-calendar-revision-event', title: '제품 점검', startsAt: response.startsAt, endsAt: response.endsAt, isWholeDay: false, ...(participants.length ? {} : { isOpenToCompany: true }), participants, updatedAt: loadingFixtureTime, source: 'event', readOnly: false }];
		await route.fulfill({ json: { result: { count: events.length, events } } });
	});
	return response;
}

async function geometry(page: Page) {
	return page.evaluate(() => {
		const rect = (selector: string) => {
			const box = document.querySelector(selector)!.getBoundingClientRect();
			return { x: box.x, y: box.y, width: box.width, height: box.height };
		};
		return { toolbar: rect('.calendar-toolbar'), stage: rect('.calendar-stage'), scrollX, scrollY };
	});
}

async function capture(page: Page, scene: string, width: number) {
	await page.evaluate(async () => {
		await document.fonts.ready;
		(document.activeElement as HTMLElement | null)?.blur();
		await Promise.all(document.getAnimations().filter(animation => animation.playState === 'running' && Number.isFinite(animation.effect?.getComputedTiming().endTime)).map(animation => animation.finished.catch(() => {})));
		window.scrollTo(0, 0);
		await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
	});
	expect(await page.evaluate(() => matchMedia('(prefers-reduced-motion: reduce)').matches)).toBe(true);
	const beforeCapture = await geometry(page);
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
	if (directory) {
		await mkdir(directory, { recursive: true });
		const path = `${directory}/calendar-r2-${scene}-${width}-${before ? 'before' : 'after'}`;
		await page.screenshot({ path: `${path}.png`, animations: 'allow', caret: 'initial' });
		const afterCapture = await geometry(page);
		expect(afterCapture).toEqual(beforeCapture);
		await writeFile(`${path}.json`, JSON.stringify({ viewport: page.viewportSize(), url: page.url(), reducedMotion: true, beforeCapture, afterCapture }, null, 2));
	}
	return beforeCapture;
}

for (const width of [1280, 390, 320]) {
	for (const view of ['month', 'week', 'day']) {
		test(`calendar revised ${view} loading keeps known geometry at ${width}px`, async ({ page }) => {
			await page.setViewportSize({ width, height: 900 });
			const response = await prepare(page, view);
			response.gate.block();
			await page.goto('/example-co/calendar?date=2026-10-06');
			await expect.poll(() => response.gate.reads).toBeGreaterThan(0);
			await expect(page.locator('.calendar-stage')).toHaveAttribute('aria-busy', 'true');
			if (!before) {
				await expect(page.locator('[data-calendar-loading-status]')).toBeVisible();
				const status = await page.locator('[data-calendar-loading-status]').boundingBox();
				const navigation = width < 640 ? await page.locator('.internkim-app-mobile-navigation').boundingBox() : null;
				expect(status).not.toBeNull();
				expect(status!.y + status!.height).toBeLessThanOrEqual(navigation?.y ?? 900);
				expect(await page.locator('[data-calendar-loading-status] svg').evaluate(node => getComputedStyle(node).animationName)).toBe('none');
				await expect(page.locator('[data-calendar-loading-cell]')).toHaveCount(0);
				await expect(page.locator('.calendar-stage [data-slot="empty"]:visible')).toHaveCount(0);
				await expect(page.locator('[data-calendar-event-id]')).toHaveCount(0);
			}
			const pending = await capture(page, `${view}-pending`, width);
			response.gate.release();
			await expect(page.locator('.calendar-stage').getByRole('button', { name: /제품 점검/ }).first()).toBeVisible();
			await expect(page.locator('.calendar-stage')).toHaveAttribute('aria-busy', 'false');
			const ready = await capture(page, `${view}-loaded`, width);
			expect(ready.toolbar).toEqual(pending.toolbar);
			expect(ready.stage).toEqual(pending.stage);
		});
	}
}

for (const width of [1280, 390]) {
	for (const view of ['month', 'week', 'day']) {
		test(`calendar settled empty ${view} preserves the editable grid at ${width}px`, async ({ page }) => {
			await page.setViewportSize({ width, height: 900 });
			await prepare(page, view, true);
			await page.goto('/example-co/calendar?date=2026-10-06');
			await expect(page.locator('.calendar-stage')).toHaveAttribute('aria-busy', 'false');
			await expect(page.locator('[data-calendar-date]').first()).toBeAttached();
			await expect(page.locator('[data-calendar-event-id]')).toHaveCount(0);
			if (view === 'day' && width === 1280) await expect(page.locator('aside [data-slot="empty"]')).toBeVisible();
			await capture(page, `${view}-empty`, width);
		});
	}
}

for (const width of [1280, 390]) {
	test(`calendar empty participant filter has a reset and search-empty at ${width}px`, async ({ page }) => {
		await page.setViewportSize({ width, height: 900 });
		const response = await prepare(page, 'month');
		await page.goto('/example-co/calendar?date=2026-10-06');
		await expect(page.getByRole('button', { name: /제품 점검/ }).first()).toBeVisible();
		if (width < 640) await page.getByRole('button', { name: /일정 도구$/ }).click();
		await page.getByRole('combobox', { name: '참여자 선택', exact: true }).click();
		await page.getByRole('combobox').last().fill('존재하지않는참여자');
		await expect(page.getByText('결과가 없습니다', { exact: true })).toBeVisible();
		if (!before) await expect(page.locator('[data-slot="command-empty"] [data-slot="empty"]')).toBeVisible();
		await capture(page, 'participant-search-empty', width);
		await page.getByRole('combobox').last().fill('박예시2');
		await page.getByRole('option', { name: /박예시2/ }).click();
		await page.keyboard.press('Escape');
		await expect(page.getByRole('button', { name: /제품 점검/ })).toHaveCount(0);
		if (!before) await expect(page.getByTestId('calendar-filter-empty')).toBeVisible();
		await capture(page, 'filter-empty', width);
		if (!before) {
			if (width >= 640) {
				response.gate.block();
				await page.getByRole('button', { name: '새로고침', exact: true }).first().click();
				await expect.poll(() => response.gate.reads).toBeGreaterThan(1);
				await expect(page.getByTestId('calendar-filter-empty')).toBeVisible();
				await expect(page.locator('[data-calendar-loading-status]')).toBeVisible();
				response.gate.release();
				await expect(page.locator('[data-calendar-loading-status]')).toHaveCount(0);
			}
			await page.getByRole('button', { name: '필터 해제', exact: true }).click();
			await expect(page.getByRole('button', { name: /제품 점검/ }).first()).toBeVisible();
		}
	});
}

test('calendar refresh and permission failures are never presented as empty', async ({ page }) => {
	await page.setViewportSize({ width: 1280, height: 900 });
	const response = await prepare(page, 'day');
	await page.goto('/example-co/calendar?date=2026-10-06');
	await expect(page.getByRole('button', { name: /제품 점검/ }).first()).toBeVisible();
	response.gate.block();
	await page.getByRole('button', { name: '새로고침', exact: true }).first().click();
	await expect.poll(() => response.gate.reads).toBeGreaterThan(1);
	await expect(page.getByRole('button', { name: /제품 점검/ }).first()).toBeVisible();
	await capture(page, 'refresh-pending', 1280);
	response.status = 503;
	response.gate.release();
	await expect(page.locator('.calendar-load-warning')).toBeVisible();
	await expect(page.getByRole('button', { name: /제품 점검/ }).first()).toBeVisible();
	await expect(page.locator('.calendar-stage [data-slot="empty"]:visible')).toHaveCount(0);
	await capture(page, 'refresh-error', 1280);
	response.status = 403;
	await page.getByRole('button', { name: '새로고침', exact: true }).first().click();
	await expect(page.getByText('일정 조회 권한이 없습니다', { exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: /제품 점검/ })).toHaveCount(0);
	await expect(page.locator('.calendar-stage [data-slot="empty"]:visible')).toHaveCount(0);
	await expect(page.locator('[data-calendar-loading-status]')).toHaveCount(0);
	await capture(page, 'permission-error', 1280);
});

test('a matching event in another cached month does not hide the visible-week empty state', async ({ page }) => {
	test.skip(before, 'New displayed-range regression assertion only');
	const response = await prepare(page, 'week');
	response.startsAt = '2026-11-10T01:00:00.000Z';
	response.endsAt = '2026-11-10T02:00:00.000Z';
	response.participantID = 'sample-1';
	await page.goto('/example-co/calendar?date=2026-10-06');
	await expect(page.locator('.calendar-stage')).toHaveAttribute('aria-busy', 'false');
	await page.getByRole('combobox', { name: '참여자 선택', exact: true }).click();
	await page.getByRole('option', { name: /박예시2/ }).click();
	await expect(page.getByTestId('calendar-filter-empty')).toBeVisible();
	await page.goto('/example-co/calendar?date=2026-11-10');
	await expect(page.getByRole('button', { name: /제품 점검/ }).first()).toBeVisible();
	await page.getByRole('combobox', { name: '참여자 선택', exact: true }).click();
	await page.getByRole('option', { name: /박예시2/ }).click();
	await expect(page.getByTestId('calendar-filter-empty')).toHaveCount(0);
});
