import { createClient } from '@supabase/supabase-js';
import { asMember, controlPlane, provisionCompany } from '../src/lib/server/control-plane';

// save_flow_task is what the board calls to write a task, and it is granted to
// authenticated only, so this signs in as a member rather than using the service
// key. It builds its own company so the check never touches anyone's real work.

const projectURL = process.env.SUPABASE_URL ?? '';
const serviceRoleKey = process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '';
const publishableKey = process.env.SUPABASE_PUBLISHABLE_KEY ?? '';
if (!projectURL || !serviceRoleKey || !publishableKey) {
	throw new Error('needs SUPABASE_URL, SUPABASE_SECRET_KEY and SUPABASE_PUBLISHABLE_KEY');
}

const admin = controlPlane({ projectURL, serviceRoleKey });
const stamp = new Date().toISOString().replaceAll(/[^0-9]/g, '').slice(0, 14);
const email = `flow-save-check-${stamp}@example.test`;

const company = await provisionCompany(
	admin,
	{
		name: 'Flow save check',
		slug: `flow-save-check-${stamp}`,
		country: 'KR',
		locale: 'ko',
		timezone: 'Asia/Seoul'
	},
	email
);

const account = await admin.auth.admin.createUser({
	email,
	password: 'seed-password',
	email_confirm: true
});
if (account.error) throw new Error(`account: ${account.error.message}`);
const linked = await admin
	.from('member')
	.update({ user_id: account.data.user.id })
	.eq('id', company.adminMemberID);
if (linked.error) throw new Error(`link: ${linked.error.message}`);

const anonymous = createClient(projectURL, publishableKey);
const session = await anonymous.auth.signInWithPassword({ email, password: 'seed-password' });
if (session.error) throw new Error(`sign in: ${session.error.message}`);
const caller = asMember({ projectURL, publishableKey }, session.data.session?.access_token ?? '');

const saved = await caller.rpc('save_flow_task', {
	target_task_id: null,
	target_title: '업무 저장 확인 (이샘플)',
	target_status: 'todo',
	target_note: '',
	target_business: '',
	target_type: '',
	target_size: '',
	target_starts_at: null,
	target_ends_at: null,
	target_write_dates: false,
	target_requester_id: null,
	target_participant_ids: [company.adminMemberID],
	target_parent_task_id: null
});

if (saved.error) {
	console.error(`save_flow_task FAILED: ${saved.error.message}`);
} else {
	const { data: row } = await admin
		.from('task')
		.select('id, title, created_at')
		.eq('id', saved.data)
		.single();
	console.log(`save_flow_task ok: ${row?.title} created_at=${row?.created_at}`);
}

await admin.from('company').delete().eq('id', company.companyID);
await admin.auth.admin.deleteUser(account.data.user.id);
console.log('test company and account removed');
