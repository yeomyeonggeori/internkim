<script lang="ts">
	import { browser } from '$app/environment';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import { calendarText } from './text';
	import { calendarRefresh } from './refresh-signal.svelte';

	const text = createPageText(calendarText);
	const iframeKey = $derived(calendarRefresh.ticks);
	let embedQuery = $state('');

	onMount(() => {
		embedQuery = window.location.search;
	});
</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main class="flex min-h-0 flex-1 flex-col bg-background text-foreground">
	{#key iframeKey}
		<iframe title={text.title} src={browser ? `/calendar/embed${embedQuery}` : '/calendar/embed'} class="min-h-0 flex-1 border-0 bg-background"></iframe>
	{/key}
</main>
