import type { HandleClientError } from '@sveltejs/kit';

import { scheduleModuleLoadRecovery } from '$lib/module-load-recovery';

export const handleError: HandleClientError = ({ error, message }) => {
	console.error(error);
	if (scheduleModuleLoadRecovery(error, message)) return { message: '앱 새 버전을 불러오는 중입니다.' };
	return { message };
};
