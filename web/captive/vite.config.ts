import tailwindcss from '@tailwindcss/vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';

function pathFromHere(relativePath: string): string {
	return fileURLToPath(new URL(relativePath, import.meta.url));
}

export default defineConfig({
	root: pathFromHere('.'),
	base: '/',
	publicDir: false,
	plugins: [tailwindcss(), svelte({ configFile: false })],
	resolve: { alias: { $lib: pathFromHere('../src/lib') } },
	build: {
		outDir: pathFromHere('../../internal/boxwifi/captive'),
		emptyOutDir: true,
		assetsDir: '',
		rollupOptions: {
			output: { entryFileNames: 'captive.js', assetFileNames: '[name][extname]' }
		}
	}
});
