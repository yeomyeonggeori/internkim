<script lang="ts">
	import AttendanceLeavePolicySettings from './attendance-leave-policy-settings.svelte';
	import CompanyHolidaySettings from './company-holiday-settings.svelte';
	import type { AdminPageText } from './admin-types';

	type AttendanceSettingsSectionProps = {
		adminBaseURL: string;
		text: AdminPageText;
	};

	let { adminBaseURL, text }: AttendanceSettingsSectionProps = $props();
</script>

<div class="grid gap-5">
	<CompanyHolidaySettings {adminBaseURL} {text} />
	<AttendanceLeavePolicySettings {adminBaseURL} {text} />
</div>
