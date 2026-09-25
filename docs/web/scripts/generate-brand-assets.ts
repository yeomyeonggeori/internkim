import { copyFileSync } from 'node:fs';
import { resolve } from 'node:path';

const scriptDirectory = import.meta.dirname;
const repositoryRoot = resolve(scriptDirectory, '../../..');
const publicDirectory = resolve(scriptDirectory, '../public');
const assetNames = [
  'logo.svg',
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
