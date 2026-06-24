<script lang="ts" module>
	export type Language = {
		/** Language code (e.g., 'en', 'de') */
		code: string;
		/** Display name (e.g., 'English', 'Deutsch') */
		label: string;
	};

	export type LanguageSwitcherProps = {
		/** List of available languages */
		languages: Language[];

		/** Current selected language code */
		value?: string;

		/** Dropdown alignment */
		align?: 'start' | 'center' | 'end';

		/** Button variant */
		variant?: 'outline' | 'ghost';

		/** Called when the language changes */
		onChange?: (code: string) => void;

		ariaLabel?: string;

		class?: string;
	};
</script>

<script lang="ts">
	import GlobeIcon from '@lucide/svelte/icons/globe';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { buttonVariants } from '$lib/components/ui/button';
	import { cn } from '$lib/utils.js';

	let {
		languages = [],
		value = $bindable(''),
		align = 'end',
		variant = 'outline',
		onChange,
		ariaLabel = 'Change language',
		class: className
	}: LanguageSwitcherProps = $props();

	let selectedValue = $state('');

	$effect(() => {
		const fallbackValue = languages[0]?.code ?? '';
		const nextValue = value || fallbackValue;
		if (selectedValue !== nextValue) selectedValue = nextValue;
	});

	function selectLanguage(code: string) {
		selectedValue = code;
		value = code;
		onChange?.(code);
	}
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger
		class={cn(buttonVariants({ variant, size: 'icon' }), className)}
		aria-label={ariaLabel}
	>
		<GlobeIcon class="size-4" />
		<span class="sr-only">{ariaLabel}</span>
	</DropdownMenu.Trigger>
	<DropdownMenu.Content {align}>
		<DropdownMenu.RadioGroup bind:value={selectedValue} onValueChange={selectLanguage}>
			{#each languages as language (language.code)}
				<DropdownMenu.RadioItem value={language.code}>
					{language.label}
				</DropdownMenu.RadioItem>
			{/each}
		</DropdownMenu.RadioGroup>
	</DropdownMenu.Content>
</DropdownMenu.Root>
