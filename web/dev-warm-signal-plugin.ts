import { glob } from 'node:fs/promises';
import type { Plugin, ViteDevServer } from 'vite';

export const warmPath = '/__warm';

async function urlsOf(patterns: string[], root: string): Promise<string[]> {
	const files = await Array.fromAsync(glob(patterns, { cwd: root }));
	return files.map((file) => `/${file}`);
}

async function warmTheBrowserModules(server: ViteDevServer): Promise<void> {
	const client = server.environments.client;
	const urls = await urlsOf(client.config.dev.warmup, server.config.root);
	await Promise.all(urls.map((url) => client.warmupRequest(url)));
	await client.waitForRequestsIdle();
}

async function loadTheRenderingModules(server: ViteDevServer): Promise<void> {
	const urls = await urlsOf(server.environments.ssr.config.dev.warmup, server.config.root);
	await Promise.all(urls.map((url) => server.ssrLoadModule(url)));
}

export function devWarmSignalPlugin(): Plugin {
	return {
		name: 'internkim-dev-warm-signal',
		apply: 'serve',
		configureServer(server) {
			let warm: Promise<unknown> | undefined;
			server.middlewares.use(warmPath, (_request, response) => {
				warm ??= Promise.all([warmTheBrowserModules(server), loadTheRenderingModules(server)]);
				warm.then(
					() => response.end('warm'),
					(error: unknown) => {
						response.statusCode = 500;
						response.end(String(error));
					}
				);
			});
		}
	};
}
