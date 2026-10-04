import { copyFileSync, mkdirSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';

const scriptDirectory = import.meta.dirname;
const repositoryRoot = resolve(scriptDirectory, '../../..');
const publicDirectory = resolve(scriptDirectory, '../public');
const fontDirectory = resolve(publicDirectory, 'fonts');
const sourceFontDirectory = resolve(repositoryRoot, 'web/src/lib/fonts');
const assetNames = [
  'mark.svg',
  'favicon.svg',
  'favicon.png',
  'favicon-192.png',
  'favicon-512.png',
];

for (const assetName of assetNames) {
  copyFileSync(
    resolve(repositoryRoot, 'web/static', assetName),
    resolve(publicDirectory, assetName),
  );
}

mkdirSync(fontDirectory, { recursive: true });

const fontNames = readdirSync(sourceFontDirectory).filter((fontName) =>
  fontName.endsWith('.woff2'),
);

if (fontNames.length === 0) {
  throw new Error(`No web fonts found in ${sourceFontDirectory}`);
}

for (const fontName of fontNames) {
  copyFileSync(
    resolve(sourceFontDirectory, fontName),
    resolve(fontDirectory, fontName),
  );
}
