import themeSource from '../../theme.css?raw';
import type { ThemeName } from './workspace.svelte';

export type ColorToken = { name: string; foreground: string };

export type RadiusToken = { name: string; definition: string };

const declarationPattern = /--([\w-]+):\s*([^;]+);/g;

function declarationsIn(block: string): Map<string, string> {
	return new Map([...block.matchAll(declarationPattern)].map((match) => [match[1], match[2].trim()]));
}

function blockAfter(selectorPattern: RegExp): string {
	const match = themeSource.match(selectorPattern);
	return match?.[1] ?? '';
}

const themeBlock = declarationsIn(blockAfter(/@theme inline\s*\{([^}]*)\}/));
const lightValues = declarationsIn(blockAfter(/:root[^{]*\{([^}]*)\}/));
const darkValues = declarationsIn(blockAfter(/\n\s*\.dark\s*\{([^}]*)\}/));

const colorNames = [...themeBlock.keys()]
	.filter((key) => key.startsWith('color-'))
	.map((key) => key.slice('color-'.length))
	.filter((name) => lightValues.has(name));

function foregroundNameOf(name: string): string {
	return name === 'background' ? 'foreground' : `${name}-foreground`;
}

const foregroundNames = new Set(colorNames.map(foregroundNameOf).filter((name) => colorNames.includes(name)));

export const colorTokens: ColorToken[] = colorNames
	.filter((name) => !foregroundNames.has(name))
	.map((name) => ({
		name,
		foreground: foregroundNames.has(foregroundNameOf(name)) ? foregroundNameOf(name) : ''
	}));

export function colorValue(name: string, theme: ThemeName): string {
	const values = theme === 'dark' ? darkValues : lightValues;
	return values.get(name) ?? lightValues.get(name) ?? '';
}

export const colorTokenNames = new Set(colorNames);

export const radiusTokens: RadiusToken[] = [...themeBlock.entries()]
	.filter(([key]) => key.startsWith('radius-'))
	.map(([key, definition]) => ({ name: key.slice('radius-'.length), definition }));

export const baseRadius = lightValues.get('radius') ?? '';

export const fontFamily = themeBlock.get('font-sans') ?? '';
