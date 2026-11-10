<script lang="ts">
	import ArrowRightIcon from '@lucide/svelte/icons/arrow-right';
	import CheckIcon from '@lucide/svelte/icons/check';
	import { onMount } from 'svelte';
	import { hostInstallCommands } from '$lib/company/host-setup';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Code from '$lib/components/ui/code';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import LandingDay from '$lib/landing/landing-day.svelte';
	import LandingDemo from '$lib/landing/landing-demo.svelte';
	import LandingHeader from '$lib/landing/landing-header.svelte';
	import LandingInvestors from '$lib/landing/landing-investors.svelte';
	import LandingOwnership from '$lib/landing/landing-ownership.svelte';
	import { heldWhileSignedInAttribute, holdWhileSignedInScript } from '$lib/landing/hold-while-signed-in';
	import { docsURL, makerURL, repositoryURL, signInPath, startCompanyPath } from '$lib/landing/links';
	import { landingText } from '$lib/landing/text';

	const text = createPageText(landingText);

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

{#snippet sectionHeading(id: string, title: string, body: string)}
	<div class="grid gap-x-12 gap-y-4 md:grid-cols-[minmax(0,5fr)_minmax(0,6fr)] md:items-end">
		<h2 {id} class="text-3xl leading-tight font-bold tracking-tight break-keep text-balance sm:text-4xl">{title}</h2>
		{#if body}<p class="text-muted-foreground max-w-xl text-lg leading-8 break-keep">{body}</p>{/if}
	</div>
{/snippet}

<div class="landing bg-background text-foreground min-h-dvh">
	<LandingHeader />

	<main>
		<section class="mx-auto grid w-full max-w-6xl items-center gap-12 px-4 pt-14 pb-20 sm:px-6 sm:pt-20 lg:grid-cols-[minmax(0,1fr)_minmax(0,31rem)] lg:pt-24 lg:pb-28">
			<div class="grid content-start gap-7">
				<h1 class="text-[2.5rem] leading-[1.15] font-bold tracking-tight break-keep text-balance sm:text-[3.4rem]">{text.hero.headline}</h1>
				<p class="text-muted-foreground max-w-xl text-lg leading-8 break-keep">{text.hero.lead}</p>
				<div class="flex flex-wrap items-center gap-3">
					{@render startButton()}
					<Button size="lg" variant="outline" href={signInPath}>{text.hero.signIn}</Button>
				</div>
				<ul class="text-muted-foreground flex flex-wrap gap-x-5 gap-y-2 text-sm">
					{#each text.hero.facts as fact (fact)}
						<li class="flex items-center gap-1.5"><CheckIcon class="size-3.5" />{fact}</li>
					{/each}
				</ul>
			</div>
			<LandingDemo />
		</section>

		<section class="mx-auto grid w-full max-w-6xl gap-10 px-4 py-20 sm:px-6 sm:py-24" aria-labelledby="landing-day">
			{@render sectionHeading('landing-day', text.day.title, text.day.lead)}
			<LandingDay />
		</section>

		<section class="bg-muted/40 border-y" aria-labelledby="landing-investors">
			<div class="mx-auto grid w-full max-w-6xl gap-12 px-4 py-20 sm:px-6 sm:py-24">
				{@render sectionHeading('landing-investors', text.investors.title, text.investors.lead)}
				<LandingInvestors />
			</div>
		</section>

		<section class="mx-auto grid w-full max-w-6xl gap-12 px-4 py-20 sm:px-6 sm:py-24" aria-labelledby="landing-ownership">
			{@render sectionHeading('landing-ownership', text.ownership.title, text.ownership.lead)}
			<LandingOwnership />
		</section>

		<div class="border-t"></div>

		<section class="mx-auto grid w-full max-w-6xl gap-10 px-4 py-20 sm:px-6 sm:py-24" aria-labelledby="landing-plans">
			{@render sectionHeading('landing-plans', text.plans.title, text.plans.lead)}
			<div class="grid gap-4">
				<Card.Root>
					<Card.Header><Card.Title class="text-lg">{text.plans.pay.title}</Card.Title></Card.Header>
					<Card.Content>
						<dl class="grid gap-6 md:grid-cols-3 md:gap-0 md:divide-x">
							{#each text.plans.pay.items as item (item.title)}
								<div class="grid content-start gap-1.5 md:px-6 md:first:pl-0 md:last:pr-0">
									<dt class="font-medium">{item.title}</dt>
									<dd class="text-muted-foreground leading-7 break-keep">{item.body}</dd>
								</div>
							{/each}
						</dl>
					</Card.Content>
				</Card.Root>
				<Card.Root class="gap-0 overflow-hidden py-0 md:grid md:grid-cols-2">
					<img src="/landing/kim-mini.webp" alt={text.plans.run.kimMini} width="1440" height="990" loading="lazy" class="aspect-[16/11] h-full w-full object-cover max-md:border-b md:border-r" />
					<div class="grid content-start gap-6 py-6">
						<Card.Header><Card.Title class="text-lg">{text.plans.run.title}</Card.Title></Card.Header>
						<Card.Content class="grid gap-4">
							<dl class="divide-y">
								{#each text.plans.run.items as item (item.title)}
									<div class="grid gap-1 py-3 first:pt-0 last:pb-0 sm:grid-cols-[7rem_minmax(0,1fr)] sm:gap-4">
										<dt class="font-medium">{item.title}</dt>
										<dd class="text-muted-foreground break-keep">{item.body}</dd>
									</div>
								{/each}
							</dl>
							<Code.Root lang="bash" hideLines code={hostInstallCommands} class="text-xs">
								<Code.CopyButton />
							</Code.Root>
							<a href={docsURL(currentLocale.value, 'self-hosting')} class="text-sm font-medium underline-offset-4 hover:underline">{text.plans.run.guide} →</a>
						</Card.Content>
					</div>
				</Card.Root>
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
			<a href={makerURL} class="hover:text-foreground mr-auto">© {new Date().getFullYear()} {text.footer.owner}. {text.footer.rights}</a>
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
