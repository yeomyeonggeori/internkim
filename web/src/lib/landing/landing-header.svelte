<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { LanguageSwitcher } from '$lib/components/ui/language-switcher';
	import { LightSwitch } from '$lib/components/ui/light-switch';
	import { currentLocale, localeOptions, setLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import GithubMark from './github-mark.svelte';
	import { docsURL, repositoryURL, signInPath } from './links';
	import { landingText } from './text';

	const text = createPageText(landingText);

	function selectLocale(code: string) {
		if (code === 'ko' || code === 'en') setLocale(code);
	}
</script>

<header class="bg-background/90 sticky top-0 z-20 border-b backdrop-blur">
	<nav class="mx-auto flex h-14 w-full max-w-6xl items-center gap-2 px-4 sm:px-6">
		<a href="/" class="mr-auto flex items-center gap-2 font-semibold">
			<img src="/favicon.svg" alt="" class="size-7 rounded-md" />
			{text.brand}
		</a>
		<Button variant="ghost" size="sm" href={docsURL(currentLocale.value)} class="max-sm:hidden">{text.navigation.docs}</Button>
		<Button variant="ghost" size="icon-sm" href={repositoryURL} aria-label={text.navigation.github}><GithubMark /></Button>
		<LanguageSwitcher
			variant="ghost"
			languages={localeOptions.map((option) => ({ code: option.value, label: option.label }))}
			value={currentLocale.value}
			ariaLabel={text.navigation.changeLanguage}
			onChange={selectLocale}
		/>
		<LightSwitch variant="ghost" />
		<Button variant="outline" size="sm" href={signInPath}>{text.navigation.signIn}</Button>
	</nav>
</header>
