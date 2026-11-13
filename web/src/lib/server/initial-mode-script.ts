import { createInitialModeExpression } from 'mode-watcher';
import { initialModePlaceholder } from './initial-mode-placeholder';

export function withInitialModeScript(html: string): string {
	return html.replace(initialModePlaceholder, `<script id="initial-mode">${createInitialModeExpression()}</script>`);
}
