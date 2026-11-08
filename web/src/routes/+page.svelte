<script lang="ts">
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
	import { onMount } from 'svelte';
	import { appSectionIcons, type AppSection } from '$lib/components/app-section-icons';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Code from '$lib/components/ui/code';
	import * as Item from '$lib/components/ui/item';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import LandingDemo from '$lib/landing/landing-demo.svelte';
	import LandingHeader from '$lib/landing/landing-header.svelte';
	import { heldWhileSignedInAttribute, holdWhileSignedInScript } from '$lib/landing/hold-while-signed-in';
	import { docsURL, installCommands, makerURL, repositoryURL, signInPath, startCompanyPath } from '$lib/landing/links';
	import { landingText } from '$lib/landing/text';

	const text = createPageText(landingText);
	const shellText = createPageText(appShellText);
	const workspaceSections: AppSection[] = ['task', 'calendar', 'attendance', 'messenger', 'mail', 'dataRoom', 'crm', 'organization', 'files'];

	onMount(() => document.documentElement.removeAttribute(heldWhileSignedInAttribute));
</script>

<svelte:head>
	<title>{text.title}</title>
	<meta name="description" content={text.description} />
	<meta property="og:title" content={text.title} />
	<meta property="og:description" content={text.description} />
	<!-- eslint-disable-next-line svelte/no-at-html-tags -->
	{@html holdWhileSignedInScript}
</svelte:head>

{#snippet startButton()}
	<Button size="lg" href={startCompanyPath} class="bg-brand hover:bg-brand/90 dark:text-background text-white">
		{text.hero.start}
		<ArrowRightIcon />
	</Button>
{/snippet}

<div class="landing bg-background text-foreground min-h-dvh">
	<LandingHeader />

	<main>
		<section class="mx-auto grid w-full max-w-6xl items-center gap-12 px-4 pt-14 pb-20 sm:px-6 sm:pt-20 lg:grid-cols-[minmax(0,1fr)_minmax(0,31rem)] lg:pt-24 lg:pb-28">
			<div class="grid content-start gap-7">
				<h1 class="text-[2.6rem] leading-[1.12] font-bold tracking-tight break-keep text-balance sm:text-6xl">{text.hero.headline}</h1>
				<p class="text-muted-foreground max-w-xl text-lg leading-8">{text.hero.lead}</p>
				<div class="flex flex-wrap items-center gap-3">
					{@render startButton()}
					<Button size="lg" variant="outline" href={signInPath}>{text.hero.signIn}</Button>
				</div>
				<a href={repositoryURL} class="text-muted-foreground hover:text-foreground w-fit text-sm underline-offset-4 hover:underline">
					{text.hero.openSource}
				</a>
			</div>
			<LandingDemo />
		</section>

		<section class="bg-muted/40 border-y" aria-labelledby="landing-workspace">
			<div class="mx-auto grid w-full max-w-6xl items-center gap-10 px-4 py-20 sm:px-6 md:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
				<div class="grid content-start gap-4">
					<h2 id="landing-workspace" class="text-3xl font-bold tracking-tight">{text.workspace.title}</h2>
					<p class="text-muted-foreground leading-7">{text.workspace.body}</p>
					<p class="text-sm font-medium">{text.workspace.channels}</p>
				</div>
				<Item.Group class="grid grid-cols-2 gap-2 sm:grid-cols-3">
					{#each workspaceSections as section (section)}
						{@const SectionIcon = appSectionIcons[section]}
						<Item.Root variant="outline" class="bg-card">
							<Item.Media variant="icon"><SectionIcon /></Item.Media>
							<Item.Content><Item.Title>{shellText[section]}</Item.Title></Item.Content>
						</Item.Root>
					{/each}
				</Item.Group>
			</div>
		</section>

		<section class="mx-auto grid w-full max-w-6xl gap-10 px-4 py-20 sm:px-6" aria-labelledby="landing-reasons">
			<h2 id="landing-reasons" class="text-3xl font-bold tracking-tight">{text.reasons.title}</h2>
			<div class="grid gap-8 md:grid-cols-3">
				{#each text.reasons.items as reason (reason.title)}
					<div class="grid content-start gap-2 border-t pt-5">
						<h3 class="text-lg font-semibold">{reason.title}</h3>
						<p class="text-muted-foreground leading-7">{reason.body}</p>
					</div>
				{/each}
			</div>
		</section>

		<section class="mx-auto grid w-full max-w-6xl gap-10 px-4 pb-20 sm:px-6" aria-labelledby="landing-trust">
			<h2 id="landing-trust" class="text-3xl font-bold tracking-tight">{text.trust.title}</h2>
			<div class="grid gap-3 md:grid-cols-3">
				{#each text.trust.items as item (item.title)}
					<Card.Root>
						<Card.Header>
							<Card.Title>{item.title}</Card.Title>
							<Card.Description>{item.body}</Card.Description>
						</Card.Header>
					</Card.Root>
				{/each}
			</div>
		</section>

		<section class="border-t" aria-labelledby="landing-hosting">
			<div class="mx-auto grid w-full max-w-6xl gap-10 px-4 py-20 sm:px-6">
				<h2 id="landing-hosting" class="text-3xl font-bold tracking-tight">{text.hosting.title}</h2>
				<div class="grid gap-3 md:grid-cols-[2fr_3fr]">
					{#each text.hosting.items as option, index (option.label)}
						<Card.Root class="min-w-0">
							<Card.Header>
								<Card.Description>{option.label}</Card.Description>
								<Card.Title>{option.title}</Card.Title>
								<Card.Description>{option.body}</Card.Description>
							</Card.Header>
							{#if index === 1}
								<Card.Content>
									<Code.Root lang="bash" hideLines code={installCommands.join('\n')}>
										<Code.CopyButton />
									</Code.Root>
								</Card.Content>
							{/if}
						</Card.Root>
					{/each}
				</div>
				<p class="text-muted-foreground text-sm">
					{text.hosting.selfHosting} ·
					<a href={docsURL(currentLocale.value, 'self-hosting')} class="text-foreground font-medium underline-offset-4 hover:underline">{text.hosting.guide}</a>
				</p>
			</div>
		</section>

		<section class="bg-muted/40 border-t">
			<div class="mx-auto grid w-full max-w-6xl justify-items-start gap-8 px-4 py-24 sm:px-6">
				<h2 class="max-w-2xl text-4xl leading-[1.2] font-bold tracking-tight break-keep text-balance">{text.closing.title}</h2>
				<div class="flex flex-wrap items-center gap-3">
					{@render startButton()}
					<Button size="lg" variant="ghost" href={docsURL(currentLocale.value, 'quickstart')}>{text.closing.quickstart}</Button>
				</div>
			</div>
		</section>
	</main>

	<footer class="border-t">
		<div class="text-muted-foreground mx-auto flex w-full max-w-6xl flex-wrap items-center gap-x-6 gap-y-2 px-4 py-8 text-sm sm:px-6">
			<a href={makerURL} class="hover:text-foreground mr-auto">{text.footer.madeBy}</a>
			<a href={docsURL(currentLocale.value)} class="hover:text-foreground">{text.navigation.docs}</a>
			<a href={repositoryURL} class="hover:text-foreground">{text.navigation.github}</a>
			<span>{text.footer.license}</span>
		</div>
	</footer>
</div>

<style>
	:global(html[data-landing-held]) .landing {
		visibility: hidden;
	}
</style>
