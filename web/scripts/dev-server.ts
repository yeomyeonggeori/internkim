//   bun run web/scripts/dev-server.ts --port 5185

import { fileURLToPath } from 'node:url';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const port = argument('port') ?? '5173';

const running = Bun.spawn(['bunx', 'vite', 'dev', '--port', port], {
	cwd: fileURLToPath(new URL('..', import.meta.url)),
	env: process.env,
	stdout: 'inherit',
	stderr: 'inherit',
});

process.on('SIGINT', () => running.kill());
process.on('SIGTERM', () => running.kill());
await running.exited;
