import { readFileSync } from 'node:fs';
import { describe, expect, test } from 'bun:test';
import { backgroundTokenOf, type ThemeSelector } from './background-token';

type Mode = { selector: ThemeSelector; constant: string };

const modes: Mode[] = [
	{ selector: ':root', constant: 'LIGHT_BACKGROUND' },
	{ selector: '.dark', constant: 'DARK_BACKGROUND' }
];

function windowColourOf(rust: string, constant: string): string {
	const match = rust.match(new RegExp(`${constant}: Color = Color\\(0x(\\w{2}), 0x(\\w{2}), 0x(\\w{2}), 0xFF\\)`));
	if (!match) throw new Error(`no ${constant} colour`);
	return (match[1] + match[2] + match[3]).toUpperCase();
}

describe('the desktop window background', () => {
	const css = readFileSync('src/theme.css', 'utf8');
	const rust = readFileSync('src-tauri/src/window_background.rs', 'utf8');

	for (const mode of modes) {
		test(`matches the web --background token under ${mode.selector}`, () => {
			expect(windowColourOf(rust, mode.constant)).toBe(backgroundTokenOf(css, mode.selector));
		});
	}
});
