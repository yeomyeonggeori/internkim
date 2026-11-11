<script lang="ts">
	import SettingsSectionPage from '../settings-section-page.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import AttendanceWorkSettingsSection from '../../admin/attendance-work-settings-section.svelte';
	import AttendanceLeavePolicySettings from '../../admin/attendance-leave-policy-settings.svelte';
	import { adminText } from '../../admin/text';

	const attendanceSettingsText = createPageText(adminText);
</script>

<SettingsSectionPage section="attendance">
	<AttendanceWorkSettingsSection adminBaseURL="" text={attendanceSettingsText} />
	<AttendanceLeavePolicySettings adminBaseURL="" text={attendanceSettingsText} />
</SettingsSectionPage>
