import { randomUUID } from 'node:crypto';
import { open, rename } from 'node:fs/promises';

export async function writeDurably(path: string, content: string): Promise<void> {
	const writingPath = `${path}.${randomUUID()}.writing`;
	const file = await open(writingPath, 'w');
	try {
		await file.writeFile(content);
		await file.sync();
	} finally {
		await file.close();
	}
	await rename(writingPath, path);
}
