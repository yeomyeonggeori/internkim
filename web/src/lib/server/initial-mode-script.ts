import { createInitialModeExpression } from 'mode-watcher';

const placeholder = '<script id="initial-mode"></script>';

export function withInitialModeScript(html: string): string {
	return html.replace(placeholder, `<script id="initial-mode">${createInitialModeExpression()}</script>`);
}
