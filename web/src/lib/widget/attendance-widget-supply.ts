import { issuePersonalAccessToken, personalAccessTokens } from '$lib/member/personal-access-tokens';
import type { PublicAPIPermission } from '$lib/public-api-permission';
import { attendanceWidgetShell, type WidgetInstall } from './attendance-widget-bridge';

// The widget writes attendance and reads what it shows, and never removes a
// record: a lost phone's key can be revoked, but it could not have deleted
// anything with it.
export const widgetTokenPermission: PublicAPIPermission = 'write';

const widgetTokenPrefix = 'ios-widget-';

export function widgetTokenNameFor(installID: string): string {
	return `${widgetTokenPrefix}${installID.replace(/[^0-9a-zA-Z]/g, '').slice(0, 8).toLowerCase()}`;
}

// The widget holds the only copy of its key, so the member's token list is what
// says whether that key still opens anything: a revoked one is gone from the
// list, and the widget is handed a new one.
export function widgetNeedsToken(held: WidgetInstall, tokens: { name: string }[]): boolean {
	if (!held.tokenName) return true;
	return !tokens.some((token) => token.name === held.tokenName);
}

let supplying: Promise<void> | null = null;

// Signing in and the session revalidating on focus both ask at once; two
// issuances under one name would leave the phone holding the key the second
// one replaced.
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

	const tokenName = widgetTokenNameFor(held.installID);
	const token = await issuePersonalAccessToken(tokenName, widgetTokenPermission);
	await shell.widget.supply({ token, tokenName, origin: window.location.origin });
}

export async function forgetWidgetSupply(): Promise<void> {
	const shell = await attendanceWidgetShell();
	if (!shell) return;
	await shell.widget.forget();
}
