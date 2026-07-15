import { basename, join } from 'node:path';
import { mkdir } from 'node:fs/promises';
import { expect, test, type Locator, type Page } from '@playwright/test';

const mattermostURL = process.env.INTERNKIM_MATTERMOST_URL?.trim() ?? '';
const probeUsername = process.env.INTERNKIM_MATTERMOST_PROBE_USERNAME?.trim() ?? '';
const probePassword = process.env.INTERNKIM_MATTERMOST_PROBE_PASSWORD ?? '';
const directMessageChannelID = process.env.INTERNKIM_MATTERMOST_DM_CHANNEL_ID?.trim() ?? '';
const rootPostID = process.env.INTERNKIM_MATTERMOST_ROOT_POST_ID?.trim() ?? '';
const botUsername = process.env.INTERNKIM_MATTERMOST_BOT_USERNAME?.trim() ?? '';
const artifactDirectory = process.env.INTERNKIM_MATTERMOST_ARTIFACT_DIR?.trim() ?? '';
const expectedAttachments = parseStringArray(process.env.INTERNKIM_MATTERMOST_EXPECT_ATTACHMENTS);
const expectedPublicURL = process.env.INTERNKIM_MATTERMOST_EXPECT_PUBLIC_URL?.trim() ?? '';
const siteProxyURL = process.env.INTERNKIM_SITE_PROXY_URL?.trim() ?? '';
const expectedPublicText = parseStringArray(process.env.INTERNKIM_MATTERMOST_EXPECT_PUBLIC_TEXT);
const expectedPublicControls = parseStringArray(process.env.INTERNKIM_MATTERMOST_EXPECT_PUBLIC_CONTROLS);
const approvalAction = process.env.INTERNKIM_MATTERMOST_APPROVAL_ACTION?.trim() ?? '';
const teamName = process.env.INTERNKIM_MATTERMOST_TEAM_NAME?.trim() || 'internkim';
const verify = expect.configure({ timeout: 0 });

test.skip(!hasRequiredEnvironment(), 'Mattermost expensive artifact environment is not configured');

test('captures real Mattermost expensive scenario artifacts', async ({ page }) => {
	test.setTimeout(0);
	page.setDefaultTimeout(0);
	page.setDefaultNavigationTimeout(0);

	await mkdir(artifactDirectory, { recursive: true });
	await signIn(page);
	await openDirectMessage(page);
	const botReply = await waitForLatestBotReply(page);
	await verify(botReply).toBeVisible();
	await verify(botReply).not.toBeEmpty();
	await page.screenshot({ path: join(artifactDirectory, 'mattermost-dm.png'), fullPage: true });
	if (approvalAction !== '') {
		await performApprovalAction(botReply, page);
	}

	for (const expectedAttachment of expectedAttachments) {
		await saveAttachment(page, expectedAttachment);
	}
	if (expectedPublicURL !== '') {
		await verifyPublicSite(botReply, page);
	}
});

async function performApprovalAction(botReply: Locator, page: Page): Promise<void> {
	if (approvalAction !== 'approve') {
		throw new Error(`Unsupported Mattermost approval action: ${approvalAction}`);
	}
	const approvalButton = botReply.getByRole('button', { name: /approve|confirm|승인|확인/i }).first();
	await verify(approvalButton).toBeVisible();
	await page.screenshot({ path: join(artifactDirectory, 'approval-before.png'), fullPage: true });
	const actionResponse = page.waitForResponse((response) =>
		/\/api\/v4\/posts\/[^/]+\/actions\/[^/]+/.test(new URL(response.url()).pathname) && response.ok(),
	);
	await approvalButton.click();
	await actionResponse;
	await page.screenshot({ path: join(artifactDirectory, 'approval-after.png'), fullPage: true });
}

function hasRequiredEnvironment(): boolean {
	return mattermostURL !== '' && probeUsername !== '' && probePassword !== '' && artifactDirectory !== '' &&
		(directMessageChannelID !== '' || botUsername !== '');
}

async function signIn(page: Page): Promise<void> {
	await page.goto(mattermostURL, { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	await page.goto(mattermostPath('/login'), { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	const loginInput = page
		.getByRole('textbox', { name: /email or username|이메일|사용자 이름|username/i })
		.or(page.locator('input[id*="loginId" i]'))
		.first();
	await verify(loginInput).toBeVisible();
	await loginInput.fill(probeUsername);
	await page.locator('input[type="password"]').first().fill(probePassword);
	const loginResponse = page.waitForResponse(
		(response) => response.url().includes('/api/v4/users/login') && response.status() === 200,
	);
	await page.getByRole('button', { name: /sign in|log in|로그인/i }).click();
	await loginResponse;
	await page.waitForURL((url) => !url.pathname.includes('/login'));
}

async function openDirectMessage(page: Page): Promise<void> {
	const directMessagePath = rootPostID
		? `/${teamName}/pl/${encodeURIComponent(rootPostID)}`
		: directMessageChannelID
		? `/${teamName}/channels/${encodeURIComponent(directMessageChannelID)}`
		: `/${teamName}/messages/@${encodeURIComponent(botUsername)}`;
	await page.goto(mattermostPath(directMessagePath), { waitUntil: 'domcontentloaded' });
	await dismissLandingPage(page);
	await verify(page).toHaveURL(new RegExp(`/${teamName}/`));
}

async function waitForLatestBotReply(page: Page): Promise<Locator> {
	const botPosts = findBotPosts(page);
	await verify.poll(async () => botPosts.count()).toBeGreaterThan(0);
	const latestBotPost = botPosts.last();
	await verify(latestBotPost).toBeVisible();
	return latestBotPost;
}

function findBotPosts(page: Page): Locator {
	const expectedAuthor = botUsername !== '' ? escapeRegularExpression(botUsername) : 'InternKim|김인턴';
	return page.getByTestId('postView').filter({ hasText: new RegExp(expectedAuthor, 'i') });
}

async function saveAttachment(page: Page, expectedAttachment: string): Promise<void> {
	const suffixPattern = new RegExp(escapeRegularExpression(expectedAttachment), 'i');
	const attachmentPost = findBotPosts(page).filter({ hasText: suffixPattern }).last();
	await verify(attachmentPost).toBeVisible();
	await attachmentPost.hover();
	const attachmentControl = attachmentPost
		.getByRole('link', { name: suffixPattern })
		.or(attachmentPost.getByRole('button', { name: suffixPattern }))
		.or(attachmentPost.getByRole('link', { name: /download|attachment|다운로드|첨부/i }))
		.or(attachmentPost.getByRole('button', { name: /download|attachment|다운로드|첨부/i }))
		.last();
	await verify(attachmentControl).toBeVisible();
	await page.screenshot({ path: join(artifactDirectory, `attachment-${safeFilename(expectedAttachment)}.png`), fullPage: true });
	const downloadPromise = page.waitForEvent('download');
	await attachmentControl.click();
	const download = await downloadPromise;
	const filename = basename(download.suggestedFilename());
	expect(filename.toLowerCase()).toMatch(new RegExp(`${escapeRegularExpression(expectedAttachment.toLowerCase())}$`));
	await download.saveAs(join(artifactDirectory, filename));
}

async function verifyPublicSite(botReply: Locator, page: Page): Promise<void> {
	const expectedURL = new URL(expectedPublicURL);
	const publicLink = await findPublicLink(botReply, expectedURL);
	await verify(publicLink).toBeVisible();
	if (siteProxyURL !== '') {
		await routePublicSiteThroughProxy(page, expectedURL);
		const sitePage = await openPublicLink(publicLink, page, expectedURL);
		await verifyPublicSiteContent(sitePage);
		return;
	}
	const sitePage = await openPublicLink(publicLink, page, expectedURL);
	await verifyPublicSiteContent(sitePage);
}

async function openPublicLink(publicLink: Locator, page: Page, expectedURL: URL): Promise<Page> {
	const target = await publicLink.getAttribute('target');
	if (target === '_blank') {
		const [openedPage] = await Promise.all([page.waitForEvent('popup'), publicLink.click()]);
		await openedPage.waitForLoadState('domcontentloaded');
		await verify.poll(() => new URL(openedPage.url()).origin).toBe(expectedURL.origin);
		return openedPage;
	}
	await Promise.all([page.waitForURL((url) => url.origin === expectedURL.origin), publicLink.click()]);
	await page.waitForLoadState('domcontentloaded');
	return page;
}

async function routePublicSiteThroughProxy(page: Page, expectedURL: URL): Promise<void> {
	await page.context().route(`${expectedURL.origin}/**`, async (route) => {
		const publicRequestURL = new URL(route.request().url());
		const proxyRequestURL = new URL(siteProxyURL);
		proxyRequestURL.pathname = publicRequestURL.pathname;
		proxyRequestURL.search = publicRequestURL.search;
		const headers = { ...route.request().headers(), host: expectedURL.host };
		const response = await route.fetch({ url: proxyRequestURL.toString(), headers });
		await route.fulfill({ response });
	});
}

async function verifyPublicSiteContent(sitePage: Page): Promise<void> {
	await verify(sitePage.locator('body')).toBeVisible();
	await verify(sitePage.locator('body')).not.toContainText(/bad gateway|not found|starter replace|replace this starter/i);
	await verify(sitePage.locator('body')).not.toBeEmpty();
	for (const fragment of expectedPublicText) {
		await verify(sitePage.getByText(fragment, { exact: false }).first()).toBeVisible();
	}
	await sitePage.setViewportSize({ width: 1440, height: 1000 });
	await sitePage.screenshot({ path: join(artifactDirectory, 'site-desktop.png'), fullPage: true });
	await sitePage.setViewportSize({ width: 390, height: 844 });
	await sitePage.screenshot({ path: join(artifactDirectory, 'site-mobile.png'), fullPage: true });
	for (const label of expectedPublicControls) {
		const control = sitePage.getByRole('link', { name: new RegExp(escapeRegularExpression(label), 'i') })
			.or(sitePage.getByRole('button', { name: new RegExp(escapeRegularExpression(label), 'i') }))
			.first();
		await verify(control).toBeVisible();
		await control.click();
		await verify(sitePage.locator('body')).toBeVisible();
		await verify(sitePage.locator('body')).not.toContainText(/bad gateway|not found|application error/i);
		await sitePage.screenshot({ path: join(artifactDirectory, `site-control-${safeFilename(label)}.png`), fullPage: true });
	}
}

function parseStringArray(value: string | undefined): string[] {
	if (!value) return [];
	const parsed: unknown = JSON.parse(value);
	if (!Array.isArray(parsed) || parsed.some((item) => typeof item !== 'string')) {
		throw new Error('Expected a JSON string array in Mattermost expensive environment');
	}
	return parsed;
}

function safeFilename(value: string): string {
	return value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'file';
}

async function findPublicLink(botReply: Locator, expectedURL: URL): Promise<Locator> {
	const links = botReply.getByRole('link');
	const linkCount = await links.count();
	for (let index = 0; index < linkCount; index += 1) {
		const href = await links.nth(index).getAttribute('href');
		if (!href) continue;
		const linkURL = new URL(href, mattermostURL);
		if (linkURL.origin === expectedURL.origin && linkURL.pathname === expectedURL.pathname) {
			return links.nth(index);
		}
	}
	throw new Error(`Expected a rendered public site link for ${expectedURL}`);
}

async function dismissLandingPage(page: Page): Promise<void> {
	const browserLink = page.getByRole('link', { name: /view in browser/i }).first();
	if ((await browserLink.count()) > 0 && (await browserLink.isVisible().catch(() => false))) {
		await browserLink.click();
		await page.waitForLoadState('domcontentloaded');
	}
}

function mattermostPath(path: string): string {
	const url = new URL(mattermostURL);
	url.pathname = path;
	return url.toString();
}

function escapeRegularExpression(value: string): string {
	return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
