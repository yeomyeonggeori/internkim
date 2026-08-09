<script lang="ts">
	import Code from '$lib/components/ui/code/code.svelte';
	import type { SupportedLanguage } from '$lib/components/ui/code/shiki';

	let { text = '', lang = '' }: { text?: string; lang?: string } = $props();

	const known: SupportedLanguage[] = [
		'bash', 'c', 'css', 'diff', 'go', 'html', 'javascript', 'json',
		'markdown', 'python', 'rust', 'sql', 'svelte', 'typescript', 'yaml'
	];
	const aliases: Record<string, SupportedLanguage> = { js: 'javascript', ts: 'typescript', sh: 'bash', shell: 'bash', py: 'python', yml: 'yaml', md: 'markdown' };
	const asked = $derived(lang.trim().toLowerCase());
	const spoken = $derived(
		known.includes(asked as SupportedLanguage) ? (asked as SupportedLanguage) : (aliases[asked] ?? 'typescript')
	);
</script>

<Code code={text} lang={spoken} hideLines class="my-2 text-xs" />
