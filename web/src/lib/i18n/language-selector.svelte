<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { currentLocale, localeOptions, setLocale } from './locale.svelte';
</script>

<div class="flex rounded-lg border bg-background p-0.5" aria-label="Language">
	{#each localeOptions as option}
		<Button
			type="button"
			size="sm"
			variant={currentLocale.value === option.value ? 'default' : 'ghost'}
			class="h-8 px-2.5 text-xs"
			aria-pressed={currentLocale.value === option.value}
			onclick={() => setLocale(option.value)}
		>
			{option.shortLabel}
		</Button>
	{/each}
</div>
