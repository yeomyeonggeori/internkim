import { expect, test, type Page } from '@playwright/test';
import { expectMultiDayTimedProxy, expectWeekAllDayEventsDoNotOverlap } from './calendar-embed-interaction-assertions';
import { navigateEmbeddedCalendar, openCalendarEmbed } from './calendar-embed-interaction-helpers';
import { routeCalendarEvents, routeDefaultCalendarAPI } from './calendar-embed-test-utils';

test.describe('embedded calendar multi-day proxy interactions', () => {
	test.beforeEach(async ({ page }) => {
		await routeDefaultCalendarAPI(page);
	});

	test('shows multi-day timed events as editable day and week all-day proxies', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'week-multi-day-timed-event',
				title: '주간 멀티',
				startISO: '2026-06-16T11:45:00+09:00',
				endISO: '2026-06-18T12:30:00+09:00',
				isAllDay: false
			},
			{
				id: 'week-multi-day-timed-event-2',
				title: '주간 멀티 2',
				startISO: '2026-06-16T13:00:00+09:00',
				endISO: '2026-06-18T14:15:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		await expectMultiDayTimedProxy(page, 'week-multi-day-timed-event', '주간 멀티', '11:45');
		await expectMultiDayTimedProxy(page, 'week-multi-day-timed-event-2', '주간 멀티 2', '13:00');
		await expect(page.locator('.df-week-event.df-event-timed[data-event-id="week-multi-day-timed-event"]:visible')).toHaveCount(0);
		await expect(page.locator('.df-week-event.df-event-timed[data-event-id="week-multi-day-timed-event-2"]:visible')).toHaveCount(0);

		await page.locator('.calendar-multi-day-all-day-proxy', { hasText: '주간 멀티' }).first().click();
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await expect(page.locator('.df-event-detail-panel')).toHaveCount(0);

		await openCalendarEmbed(page, '일');
		await navigateEmbeddedCalendar(page, '2026-06-17');
		await expect(page.locator('.calendar-multi-day-all-day-proxy[data-event-id="week-multi-day-timed-event::multi-day-proxy"]')).toHaveCount(1);
		await expect(page.locator('.calendar-multi-day-all-day-proxy[data-event-id="week-multi-day-timed-event-2::multi-day-proxy"]')).toHaveCount(1);
		await expect(page.locator('.df-day-event.df-event-timed[data-event-id="week-multi-day-timed-event"]:visible')).toHaveCount(0);
		await expect(page.locator('.df-day-event.df-event-timed[data-event-id="week-multi-day-timed-event-2"]:visible')).toHaveCount(0);
	});

	test('keeps multi-day timed proxies from overlapping week all-day events', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'week-all-day-overlap-event',
				title: '주간 종일',
				startISO: '2026-06-16T00:00:00+09:00',
				endISO: '2026-06-19T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'week-multi-day-overlap-event',
				title: '주간 시간 다일',
				startISO: '2026-06-16T11:00:00+09:00',
				endISO: '2026-06-18T12:00:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-16');

		await expectMultiDayTimedProxy(page, 'week-multi-day-overlap-event', '주간 시간 다일', '11:00');
		await expectWeekAllDayEventsDoNotOverlap(page, 'week-all-day-overlap-event', 'week-multi-day-overlap-event::multi-day-proxy');
	});

	test('keeps compact multi-day proxy tap targets at least 24 pixels tall', async ({ page }) => {
		await page.setViewportSize({ width: 390, height: 844 });
		await routeCalendarEvents(page, [
			{
				id: 'compact-multi-day-proxy',
				title: 'Compact Multi Day Proxy',
				startISO: '2026-06-16T11:45:00+09:00',
				endISO: '2026-06-18T12:30:00+09:00',
				isAllDay: false
			},
			{
				id: 'compact-multi-day-proxy-2',
				title: 'Compact Multi Day Proxy 2',
				startISO: '2026-06-16T13:00:00+09:00',
				endISO: '2026-06-18T14:30:00+09:00',
				isAllDay: false
			}
		]);
		await openCalendarEmbed(page, '일');
		await navigateEmbeddedCalendar(page, '2026-06-17');
		const proxy = page.locator(
			'.calendar-multi-day-all-day-proxy[data-event-id="compact-multi-day-proxy::multi-day-proxy"]'
		);
		await expect(proxy).toBeVisible();
		const secondProxy = page.locator(
			'.calendar-multi-day-all-day-proxy[data-event-id="compact-multi-day-proxy-2::multi-day-proxy"]'
		);
		await expect(secondProxy).toBeVisible();

		const proxyBox = await proxy.boundingBox();
		const secondProxyBox = await secondProxy.boundingBox();
		const laneBox = await page.locator('.df-day-content-all-day-lane').boundingBox();
		expect(proxyBox?.height ?? 0).toBeGreaterThanOrEqual(24);
		expect(secondProxyBox?.height ?? 0).toBeGreaterThanOrEqual(24);
		expect(proxyBox?.y ?? 0).toBeGreaterThanOrEqual(laneBox?.y ?? 0);
		expect((proxyBox?.y ?? 0) + (proxyBox?.height ?? 0)).toBeLessThanOrEqual(secondProxyBox?.y ?? 0);
	});

	test('reflows all-day events and multi-day proxies when the calendar becomes compact', async ({ page }) => {
		await page.setViewportSize({ width: 900, height: 844 });
		await routeCalendarEvents(page, [
			{
				id: 'resize-all-day-event-1',
				title: 'Resize All Day 1',
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-18T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'resize-all-day-event-2',
				title: 'Resize All Day 2',
				startISO: '2026-06-17T00:00:00+09:00',
				endISO: '2026-06-18T00:00:00+09:00',
				isAllDay: true
			},
			{
				id: 'resize-multi-day-proxy',
				title: 'Resize Multi Day Proxy',
				startISO: '2026-06-16T11:45:00+09:00',
				endISO: '2026-06-18T12:30:00+09:00',
				isAllDay: false
			}
		]);
		await openCalendarEmbed(page, '일');
		await navigateEmbeddedCalendar(page, '2026-06-17');
		await expect(page.locator('[data-event-id="resize-multi-day-proxy::multi-day-proxy"]')).toHaveCSS('height', '16px');
		await page.waitForTimeout(1_100);

		await page.setViewportSize({ width: 600, height: 844 });

		await expect
			.poll(() => compactResizeMeasurements(page))
			.toMatchObject({
				allDayEventHeights: [24, 24],
				allEventsDoNotOverlap: true,
				allEventsFitLane: true,
				proxyFitsLane: true,
				proxyHeight: 24
			});
		await expect
			.poll(async () => (await compactResizeMeasurements(page))?.allDayContentCenterDelta)
			.toBeLessThanOrEqual(1);
	});

	test('opens a multi-day proxy editor from a synthesized accessible click', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'accessible-multi-day-proxy',
				title: 'Accessible Multi Day Proxy',
				startISO: '2026-06-16T11:45:00+09:00',
				endISO: '2026-06-18T12:30:00+09:00',
				isAllDay: false
			}
		]);
		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		const proxy = page.locator(
			'.calendar-multi-day-all-day-proxy[data-event-id="accessible-multi-day-proxy::multi-day-proxy"]'
		);
		await expect(proxy).toBeVisible();

		await proxy.focus();
		await proxy.dispatchEvent('click');

		await expect(proxy).toHaveClass(/internkim-calendar-event-focused/);
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
		await page.getByRole('dialog', { name: '일정 편집' }).getByLabel('장소').focus();
		await page.keyboard.press('Escape');
		await page.waitForTimeout(1_200);
		await expect(proxy).toBeFocused();
	});

	test('opens a trusted pointer click editor without replacing the multi-day proxy', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'pointer-multi-day-proxy',
				title: 'Pointer Multi Day Proxy',
				startISO: '2026-06-16T11:45:00+09:00',
				endISO: '2026-06-18T12:30:00+09:00',
				isAllDay: false
			}
		]);
		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		const proxy = page.locator(
			'.calendar-multi-day-all-day-proxy[data-event-id="pointer-multi-day-proxy::multi-day-proxy"]'
		);
		await expect(proxy).toBeVisible();
		await expect(proxy).toHaveAttribute('aria-pressed', 'false');
		await page.waitForTimeout(1_100);
		await proxy.evaluate((element) => {
			element.dataset.proxyIdentity = 'preserved';
			const layerElement = element.parentElement;
			if (!layerElement) return;
			let attributeMutationCount = 0;
			let contentMutationCount = 0;
			let layerMoveMutationCount = 0;
			const proxyObserver = new MutationObserver((records) => {
				for (const record of records) {
					if (record.type === 'attributes') attributeMutationCount += 1;
					if (record.type === 'childList' || record.type === 'characterData') contentMutationCount += 1;
				}
			});
			const layerObserver = new MutationObserver((records) => {
				for (const record of records) {
					if ([...record.addedNodes, ...record.removedNodes].includes(element)) layerMoveMutationCount += 1;
				}
			});
			proxyObserver.observe(element, {
				attributes: true,
				attributeFilter: ['aria-pressed', 'class', 'data-event-id', 'style'],
				characterData: true,
				childList: true,
				subtree: true
			});
			layerObserver.observe(layerElement, { childList: true });
			window.setTimeout(() => {
				proxyObserver.disconnect();
				layerObserver.disconnect();
				element.dataset.proxyAttributeMutationCount = String(attributeMutationCount);
				element.dataset.proxyContentMutationCount = String(contentMutationCount);
				element.dataset.proxyLayerMoveMutationCount = String(layerMoveMutationCount);
				element.dataset.proxyObservationComplete = 'true';
			}, 1_150);
		});

		await proxy.click();
		await expect(proxy).toHaveAttribute('data-proxy-observation-complete', 'true');

		await expect(proxy).toHaveAttribute('data-proxy-identity', 'preserved');
		await expect(proxy).toHaveClass(/internkim-calendar-event-focused/);
		await expect(proxy).toHaveAttribute('aria-pressed', 'true');
		const attributeMutationCount = Number(await proxy.getAttribute('data-proxy-attribute-mutation-count'));
		expect(attributeMutationCount).toBeGreaterThan(0);
		expect(attributeMutationCount).toBeLessThanOrEqual(2);
		await expect(proxy).toHaveAttribute('data-proxy-content-mutation-count', '0');
		await expect(proxy).toHaveAttribute('data-proxy-layer-move-mutation-count', '0');
		await expect(page.locator('.calendar-draft-popover')).toBeVisible();
	});

	test('does not open a multi-day proxy popover after a moved pointer gesture', async ({ page }) => {
		await routeCalendarEvents(page, [
			{
				id: 'week-proxy-drag-guard-event',
				title: '주간 프록시 드래그',
				startISO: '2026-06-16T11:45:00+09:00',
				endISO: '2026-06-18T12:30:00+09:00',
				isAllDay: false
			}
		]);

		await openCalendarEmbed(page, '주');
		await navigateEmbeddedCalendar(page, '2026-06-16');
		const proxySelector = '.calendar-multi-day-all-day-proxy[data-event-id="week-proxy-drag-guard-event::multi-day-proxy"]';
		await expect(page.locator(proxySelector)).toBeVisible();
		const proxyBox = await page.locator(proxySelector).evaluate((element) => {
			const rectangle = element.getBoundingClientRect();
			return { x: rectangle.left, y: rectangle.top, width: rectangle.width, height: rectangle.height };
		});
		expect(proxyBox.width).toBeGreaterThan(0);
		expect(proxyBox.height).toBeGreaterThan(0);
		const sourceX = proxyBox.x + proxyBox.width / 2;
		const sourceY = proxyBox.y + proxyBox.height / 2;

		await page.mouse.move(sourceX, sourceY);
		await page.mouse.down();
		await page.mouse.move(sourceX + 90, sourceY + 14, { steps: 8 });
		await page.mouse.up();
		await page.locator(proxySelector).click();

		await expect(page.locator('.calendar-draft-popover')).toHaveCount(0);
	});
});

async function compactResizeMeasurements(page: Page) {
	return page.evaluate(() => {
		const laneElement = document.querySelector<HTMLElement>('.df-day-content-all-day-lane');
		if (!laneElement) return null;
		const allDayEventElements = [
			laneElement.querySelector<HTMLElement>('[data-event-id="resize-all-day-event-1"].df-event'),
			laneElement.querySelector<HTMLElement>('[data-event-id="resize-all-day-event-2"].df-event')
		].filter((element): element is HTMLElement => element instanceof HTMLElement);
		const proxyElement = laneElement.querySelector<HTMLElement>('[data-event-id="resize-multi-day-proxy::multi-day-proxy"]');
		if (allDayEventElements.length !== 2 || !proxyElement) return null;
		const laneRectangle = laneElement.getBoundingClientRect();
		const eventRectangles = allDayEventElements
			.map((eventElement) => eventElement.getBoundingClientRect())
			.sort((firstRectangle, secondRectangle) => firstRectangle.top - secondRectangle.top);
		const contentEventRectangle = allDayEventElements[0].getBoundingClientRect();
		const contentRectangle = allDayEventElements[0]
			.querySelector<HTMLElement>('.calendar-event-content')
			?.getBoundingClientRect();
		const proxyRectangle = proxyElement.getBoundingClientRect();
		const allEventRectangles = [...eventRectangles, proxyRectangle].sort(
			(firstRectangle, secondRectangle) => firstRectangle.top - secondRectangle.top
		);
		return {
			allDayContentCenterDelta: contentRectangle
				? Math.round(
						Math.abs(
							contentRectangle.top + contentRectangle.height / 2 -
							(contentEventRectangle.top + contentEventRectangle.height / 2)
						)
					)
				: Number.POSITIVE_INFINITY,
			allDayEventHeights: eventRectangles.map((rectangle) => Math.round(rectangle.height)),
			allEventsDoNotOverlap: allEventRectangles
				.slice(0, -1)
				.every((rectangle, index) => rectangle.bottom <= allEventRectangles[index + 1].top),
			allEventsFitLane:
				allEventRectangles[0].top >= laneRectangle.top &&
				allEventRectangles[allEventRectangles.length - 1].bottom <= laneRectangle.bottom,
			proxyFitsLane: proxyRectangle.left >= laneRectangle.left && proxyRectangle.right <= laneRectangle.right,
			proxyHeight: Math.round(proxyRectangle.height)
		};
	});
}
