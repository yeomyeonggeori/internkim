import { colorTokenNames, radiusTokens } from './theme-tokens';

export type FrameTokens = { colors: string[]; radii: string[] };

const colorUtilityPattern =
	/^(?:bg|text|border(?:-[trblxyse])?|ring|ring-offset|outline|fill|stroke|divide|from|via|to|accent|caret|decoration|placeholder)-([a-z-]+?)(?:\/[\d.]+)?$/;
const radiusUtilityPattern = /^rounded(?:-[trblse]{1,2})?-([a-z0-9]+)$/;
const radiusNames = new Set(radiusTokens.map((token) => token.name));

function baseUtility(className: string): string {
	const utility = className.split(':').at(-1) ?? '';
	return utility.replace(/^!|!$/g, '');
}

function classesWithin(root: Element): string[] {
	const elements = [root, ...root.querySelectorAll('*')];
	return [...new Set(elements.flatMap((element) => [...element.classList].map(baseUtility)))];
}

export function tokensUsedIn(root: Element): FrameTokens {
	const utilities = classesWithin(root);
	const colors = utilities
		.map((utility) => utility.match(colorUtilityPattern)?.[1] ?? '')
		.filter((name) => colorTokenNames.has(name));
	const radii = utilities
		.map((utility) => utility.match(radiusUtilityPattern)?.[1] ?? '')
		.filter((name) => radiusNames.has(name));
	return { colors: [...new Set(colors)].sort(), radii: [...new Set(radii)].sort() };
}
