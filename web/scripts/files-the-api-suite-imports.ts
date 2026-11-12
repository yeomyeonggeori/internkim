import { Glob } from 'bun';
import { relative, resolve } from 'node:path';

const repositoryRoot = resolve(import.meta.dir, '../..');
const webDirectory = resolve(import.meta.dir, '..');
const manifest = await Bun.file(resolve(webDirectory, 'package.json')).json();
const packages = Object.keys({ ...manifest.dependencies, ...manifest.devDependencies });
const sveltekitModules = ['$app/*', '$' + 'env/*'];

const built = await Bun.build({
	entrypoints: [...new Glob('tests/api/*.ts').scanSync({ cwd: webDirectory, absolute: true })],
	root: webDirectory,
	target: 'bun',
	metafile: true,
	external: [...packages, ...packages.map((name) => `${name}/*`), ...sveltekitModules]
});
if (!built.success) {
	console.error(built.logs.join('\n'));
	process.exit(1);
}

const importedFiles = Object.keys(built.metafile.inputs).map((input) => relative(repositoryRoot, resolve(webDirectory, input)));
console.log(importedFiles.sort().join('\n'));
