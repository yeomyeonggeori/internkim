import { describe, expect, test } from 'bun:test';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

const sourceDirectory = new URL('../../../src/', import.meta.url).pathname;
const appStyles = readFileSync(new URL('../../../src/app.css', import.meta.url), 'utf8');

const layerFloor = 40;

const layerNames = [
	'--layer-overlay',
	'--layer-panel',
	'--layer-panel-popout',
	'--layer-alert-overlay',
	'--layer-alert',
	'--layer-tooltip',
	'--layer-dragged'
];

function layerValue(name: string): number {
	const declared = appStyles.match(new RegExp(`${name}:\\s*(\\d+)`));
	return declared ? Number(declared[1]) : Number.NaN;
}

function orderingOfItsOwn(source: string): string[] {
	return source
		.split(/[\s"'`]+/)
		.filter((token) => /^(.*:)?z-(\[\d+\]|\d+)$/.test(token))
		.filter((token) => !token.slice(0, token.lastIndexOf('z-')).includes(':'))
		.filter((token) => Number(token.replace(/\D/g, '')) >= layerFloor);
}

function everyComponentFile(): string[] {
	const found: string[] = [];
	for (const entry of readdirSync(sourceDirectory, { withFileTypes: true, recursive: true })) {
		if (entry.isFile() && entry.name.endsWith('.svelte')) {
			found.push(join(entry.parentPath, entry.name));
		}
	}
	return found;
}

describe('what covers what on screen', () => {
	test('app.css names every layer', () => {
		const missing = layerNames.filter((name) => Number.isNaN(layerValue(name)));
		expect(missing).toEqual([]);
	});

	test('a panel sits above the overlay that dims the page behind it', () => {
		expect(layerValue('--layer-panel')).toBeGreaterThan(layerValue('--layer-overlay'));
	});

	test('what a panel opens sits above the panel', () => {
		expect(layerValue('--layer-panel-popout')).toBeGreaterThan(layerValue('--layer-panel'));
	});

	test('an alert interrupts everything a panel opened', () => {
		expect(layerValue('--layer-alert-overlay')).toBeGreaterThan(layerValue('--layer-panel-popout'));
		expect(layerValue('--layer-alert')).toBeGreaterThan(layerValue('--layer-alert-overlay'));
	});

	test('a tooltip sits above every overlay', () => {
		const overlays = layerNames.filter((name) => name !== '--layer-tooltip' && name !== '--layer-dragged');
		expect(layerValue('--layer-tooltip')).toBeGreaterThan(Math.max(...overlays.map(layerValue)));
	});

	test('what the pointer is carrying stays under nothing', () => {
		const others = layerNames.filter((name) => name !== '--layer-dragged').map(layerValue);
		expect(layerValue('--layer-dragged')).toBeGreaterThan(Math.max(...others));
	});

	test('nothing writes a stacking order of its own above the layers', () => {
		const written = everyComponentFile()
			.map((path) => ({ path, source: readFileSync(path, 'utf8') }))
			.filter(({ source }) => orderingOfItsOwn(source).length > 0)
			.map(({ path }) => path.slice(sourceDirectory.length));
		expect(written).toEqual([]);
	});
});
