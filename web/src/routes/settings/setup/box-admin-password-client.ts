import { z } from 'zod';
import { adminPasswordStatusSchema, type AdminPasswordStatus, type ConnectedBox } from '$lib/company/box';
import { sealAdminPassword } from '$lib/company/seal-to-box';
import { askCompanyRoute } from './host-setup-client';

const statusAnswerSchema = z.object({ status: adminPasswordStatusSchema }).strict();

export async function setBoxAdminPassword(
	box: ConnectedBox,
	password: string
): Promise<{ settingID: string; status: AdminPasswordStatus }> {
	const settingID = crypto.randomUUID();
	const sealed = await sealAdminPassword(password, box, settingID);
	const answer = statusAnswerSchema.parse(await askCompanyRoute('/api/company/box/admin-password', 'PUT', { settingID, sealed }));
	return { settingID, status: answer.status };
}

export async function fetchAdminPasswordStatus(): Promise<AdminPasswordStatus> {
	return statusAnswerSchema.parse(await askCompanyRoute('/api/company/box/admin-password', 'GET')).status;
}
