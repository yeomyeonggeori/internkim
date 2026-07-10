import { expect, test } from 'bun:test';
import { resolve } from 'node:path';

test('keeps DayFlow global styles out of the attendance route', async () => {
	const attendanceSourceDirectory = resolve(import.meta.dir, '../../../src/routes/attendance');
	const sourceFiles = new Bun.Glob('**/*.{svelte,ts}').scanSync({
		cwd: attendanceSourceDirectory,
		absolute: true
	});

	for (const sourceFile of sourceFiles) {
		const source = await Bun.file(sourceFile).text();
		expect(/from\s+['"]@dayflow\/(?:core|svelte)['"]/.test(source)).toBe(false);
	}
});
