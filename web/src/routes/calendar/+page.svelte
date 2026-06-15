<script lang="ts">
	import { browser } from '$app/environment';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import type { CalendarNavigationMessage } from './calendar-navigation-message';
	import { calendarText } from './text';
	import { calendarNavigation, calendarRefresh } from './refresh-signal.svelte';

	const text = createPageText(calendarText);
	const iframeKey = $derived(`${calendarRefresh.ticks}-${currentLocale.value}`);
	let calendarFrame = $state<HTMLIFrameElement | null>(null);
	let loadedIframeKey = $state('');
	let embedQuery = $state('');

	$effect(() => {
		if (!calendarFrame?.contentWindow || loadedIframeKey !== iframeKey || !calendarNavigation.dateKey) return;
		calendarFrame.contentWindow.postMessage(
			{
				type: 'calendar-navigate',
				dateKey: calendarNavigation.dateKey
			} satisfies CalendarNavigationMessage,
			window.location.origin
		);
	});

	onMount(() => {
		embedQuery = window.location.search;
	});

	function handleCalendarFrameLoad() {
		loadedIframeKey = iframeKey;
	}
</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main class="flex min-h-0 flex-1 flex-col bg-background text-foreground">
	{#key iframeKey}
		<iframe
			bind:this={calendarFrame}
			title={text.title}
			src={browser ? `/calendar/embed${embedQuery}` : '/calendar/embed'}
			onload={handleCalendarFrameLoad}
			class="min-h-0 flex-1 border-0 bg-background"
		></iframe>
	{/key}
</main>
