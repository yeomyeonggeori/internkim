import { issuePersonalAccessToken, personalAccessTokens } from '$lib/member/personal-access-tokens';
import type { PublicAPIPermission } from '$lib/public-api-permission';
import { expiresWithin } from '$lib/token-lifetime';
import { attendanceWidgetShell, type WidgetInstall, type WidgetPlatform } from './attendance-widget-bridge';

export const widgetTokenPermission: PublicAPIPermission = 'write';

export function widgetTokenNameFor(platform: WidgetPlatform, installID: string): string {
	return `${platform}-widget-${installID.replace(/[^0-9a-zA-Z]/g, '').slice(0, 8).toLowerCase()}`;
}

const renewWithinDays = 30;

export function widgetNeedsToken(
	held: WidgetInstall,
	tokens: { name: string; expiresAt: string }[],
	now: Date = new Date()
): boolean {
	if (!held.tokenName) return true;
	const supplied = tokens.find((token) => token.name === held.tokenName);
	return !supplied || expiresWithin(supplied.expiresAt, renewWithinDays, now);
}

let supplying: Promise<void> | null = null;

export function keepWidgetSupplied(): Promise<void> {
	supplying ??= supplyWidget().finally(() => {
		supplying = null;
	});
	return supplying;
}

async function supplyWidget(): Promise<void> {
	const shell = await attendanceWidgetShell();
	if (!shell) return;

	const held = await shell.widget.install();
	if (!widgetNeedsToken(held, await personalAccessTokens())) return;

	const tokenName = widgetTokenNameFor(shell.platform, held.installID);
	const token = await issuePersonalAccessToken(tokenName, widgetTokenPermission);
	await shell.widget.supply({ token, tokenName, origin: window.location.origin });
}

export async function forgetWidgetSupply(): Promise<void> {
	const shell = await attendanceWidgetShell();
	if (!shell) return;
	await shell.widget.forget();
}
