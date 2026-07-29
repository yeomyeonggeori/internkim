<script lang="ts">
	import { browser } from '$app/environment';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import AppFloatingActionButton from '$lib/components/app-floating-action-button.svelte';
	import { pageActions } from '$lib/components/app-page-actions.svelte';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { onMount } from 'svelte';
	import type { CalendarCreateEventMessage, CalendarNavigationMessage } from './calendar-navigation-message';
	import { calendarText } from './text';
	import { bumpCalendarRefresh, calendarNavigation, calendarRefresh } from './refresh-signal.svelte';

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
		return pageActions.setRefresh(bumpCalendarRefresh);
	});

	function handleCalendarFrameLoad() {
		loadedIframeKey = iframeKey;
	}

	function createCalendarEvent(): void {
		calendarFrame?.contentWindow?.postMessage(
			{ type: 'calendar-create-event' } satisfies CalendarCreateEventMessage,
			window.location.origin
		);
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

	<AppFloatingActionButton label={text.newEvent} onclick={createCalendarEvent}>
		<PlusIcon />
	</AppFloatingActionButton>
</main>
