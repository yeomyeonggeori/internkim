import { defineConfig } from 'jsrepo';

export default defineConfig({
    registries: ['https://shadcn-svelte-extras.com/registry'],
    paths: {
        ui: 'src/lib/components/ui',
        '*': 'src/lib/components',
		lib: 'src/lib/components'
    },
});