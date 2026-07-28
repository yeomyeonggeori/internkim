<script lang="ts">
	import * as Tabs from '$lib/components/ui/tabs';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { ViewType } from '@dayflow/svelte';
	import { calendarText } from '../text';

	type CalendarViewSwitcherProps = {
		toolbarView: ViewType;
		changeCalendarView: (viewType: ViewType) => void;
	};

	let { toolbarView, changeCalendarView }: CalendarViewSwitcherProps = $props();
	const text = createPageText(calendarText);

	function selectView(value: string) {
		changeCalendarView(value as ViewType);
	}
</script>

<Tabs.Root value={toolbarView} onValueChange={selectView} aria-label={text.calendarView} class="shrink-0">
	<Tabs.List>
		<Tabs.Trigger value={ViewType.DAY}>{text.day}</Tabs.Trigger>
		<Tabs.Trigger value={ViewType.WEEK}>{text.week}</Tabs.Trigger>
		<Tabs.Trigger value={ViewType.MONTH}>{text.month}</Tabs.Trigger>
	</Tabs.List>
</Tabs.Root>
