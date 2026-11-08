begin;
create extension if not exists pgtap with schema extensions;
select plan(21);

insert into auth.users (id, email, email_confirmed_at) values
  ('71000000-0000-0000-0000-000000000001', 'invite-admin@example.com', now()),
  ('71000000-0000-0000-0000-000000000002', 'invite-guest@example.com', now()),
  ('71000000-0000-0000-0000-000000000003', 'invite-stranger@example.com', now()),
  ('71000000-0000-0000-0000-000000000004', 'invite-colleague@example.com', now());
insert into public.company (id, name, slug, country, locale, timezone, task_vocabulary) values
  ('71000000-0000-0000-0000-000000000000', 'Invitation Sample', 'invitation-sample', 'KR', 'ko', 'Asia/Seoul',
   '{"businesses":[{"name":"정부과제","color":"#2563eb"},{"name":"신규 플랫폼"},{"name":"사내 운영"}],"types":[{"name":"개발","color":"#16a34a"},{"name":"총무"}]}');
insert into public.member (id, company_id, email, user_id, status, is_admin, name) values
  ('71000000-0000-0000-0000-000000000011', '71000000-0000-0000-0000-000000000000', 'invite-admin@example.com',
   '71000000-0000-0000-0000-000000000001', 'active', true, '이샘플'),
  ('71000000-0000-0000-0000-000000000012', '71000000-0000-0000-0000-000000000000', 'invite-colleague@example.com',
   '71000000-0000-0000-0000-000000000004', 'active', false, '박예시');
insert into public.task (id, company_id, title, status, business, type) values
  ('71000000-0000-0000-0000-0000000000a1', '71000000-0000-0000-0000-000000000000', '과제 중간 보고서', 'in_progress', '정부과제', '개발'),
  ('71000000-0000-0000-0000-0000000000a2', '71000000-0000-0000-0000-000000000000', '사무실 이전 견적', 'planned', '사내 운영', '총무'),
  ('71000000-0000-0000-0000-0000000000a3', '71000000-0000-0000-0000-000000000000', '플랫폼 출시 준비', 'planned', '신규 플랫폼', null);
update public.task set parent_task_id = '71000000-0000-0000-0000-0000000000a2'
  where id = '71000000-0000-0000-0000-0000000000a1';
insert into public.task_participant (task_id, member_id) values
  ('71000000-0000-0000-0000-0000000000a1', '71000000-0000-0000-0000-000000000012'),
  ('71000000-0000-0000-0000-0000000000a2', '71000000-0000-0000-0000-000000000011');

set local role authenticated;

select set_config('request.jwt.claims', '{"sub":"71000000-0000-0000-0000-000000000004"}', true);
select throws_ok($$select public.task_invite('71000000-0000-0000-0000-000000000000', 'invite-guest@example.com', array['정부과제'])$$,
  '42501', 'only a company administrator invites people to its tasks', 'a colleague who is not an administrator cannot invite');

select set_config('request.jwt.claims', '{"sub":"71000000-0000-0000-0000-000000000001"}', true);
select throws_ok($$select public.task_invite('71000000-0000-0000-0000-000000000000', 'invite-guest@example.com', array[]::text[])$$,
  '22023', 'an invitation names at least one business', 'an invitation without a business is refused');
select throws_ok($$select public.task_invite('71000000-0000-0000-0000-000000000000', 'invite-guest@example.com', array['없는 사업'])$$,
  '22023', 'an invitation names only businesses the company files work under', 'a business the company does not have is refused');
select public.task_invite('71000000-0000-0000-0000-000000000000', ' Invite-Guest@Example.com ', array['정부과제']) as invitation_id \gset
select is((select email from public.task_invitation where id = :'invitation_id'), 'invite-guest@example.com',
  'the address is kept in one spelling');

select set_config('request.jwt.claims', '{"sub":"71000000-0000-0000-0000-000000000002"}', true);
select is(public.task_invitation_board('71000000-0000-0000-0000-000000000000'), null,
  'an invitation not yet accepted shows nothing');
select is((select count(*) from public.task_invitation), 0::bigint, 'a guest cannot read the invitation table');

select set_config('request.jwt.claims', '{"sub":"71000000-0000-0000-0000-000000000003"}', true);
select throws_ok(format('select public.task_invitation_accept(%L)', :'invitation_id'),
  '42501', 'this invitation is not open to the signed-in email', 'another address cannot accept the invitation');

select set_config('request.jwt.claims', '{"sub":"71000000-0000-0000-0000-000000000002"}', true);
select is(public.task_invitation_accept(:'invitation_id'), '71000000-0000-0000-0000-000000000000'::uuid,
  'the invited address accepts and learns the company');
select is((select count(*) from public.member where user_id = auth.uid()), 0::bigint, 'accepting makes no company member');
select is((select count(*) from public.task where company_id = '71000000-0000-0000-0000-000000000000'), 0::bigint,
  'the guest still reads no task row directly');

select is(
  (select array_agg(task ->> 'title' order by task ->> 'title')
   from jsonb_array_elements(public.task_invitation_board('71000000-0000-0000-0000-000000000000') -> 'tasks') as task),
  array['과제 중간 보고서'], 'the guest sees the invited business''s tasks unmasked, and nothing else');
select is(public.task_invitation_board('71000000-0000-0000-0000-000000000000') -> 'businesses',
  '[{"name":"정부과제","color":"#2563eb"}]'::jsonb, 'only the invited business is named');
select is(public.task_invitation_board('71000000-0000-0000-0000-000000000000') -> 'types',
  '[{"name":"개발","color":"#16a34a"}]'::jsonb, 'only the types of shown tasks are named');
select is(public.task_invitation_board('71000000-0000-0000-0000-000000000000') -> 'counts',
  '{"in_progress":1}'::jsonb, 'counts cover only the invited business');
select ok(not (public.task_invitation_board('71000000-0000-0000-0000-000000000000') -> 'tasks' -> 0) ? 'parentID',
  'a parent in another business is not revealed');
select is(public.task_invitation_board('71000000-0000-0000-0000-000000000000') -> 'people',
  '{"71000000-0000-0000-0000-000000000012":{"name":"박예시"}}'::jsonb,
  'only the people on shown tasks are named');
select ok(public.task_invitation_board('71000000-0000-0000-0000-000000000000')::text !~ '(사내 운영|신규 플랫폼|사무실 이전|플랫폼 출시|이샘플)',
  'nothing of the businesses left out appears anywhere in the answer');

select set_config('request.jwt.claims', '{"sub":"71000000-0000-0000-0000-000000000001"}', true);
update public.task set business = '사내 운영' where id = '71000000-0000-0000-0000-0000000000a1';

select set_config('request.jwt.claims', '{"sub":"71000000-0000-0000-0000-000000000002"}', true);
select is(public.task_invitation_board('71000000-0000-0000-0000-000000000000') -> 'tasks', '[]'::jsonb,
  'a task moved to another business leaves the guest''s board at the next read');

select set_config('request.jwt.claims', '{"sub":"71000000-0000-0000-0000-000000000001"}', true);
select throws_ok($$select public.task_vocabulary_save('{"businesses":[{"name":"신규 플랫폼"},{"name":"사내 운영"}],"types":[{"name":"개발"},{"name":"총무"}]}')$$,
  '2BP01', 'a business an open invitation shares cannot be deleted', 'a business an open invitation shares stays in the vocabulary');
select public.task_invitation_revoke(:'invitation_id');

select set_config('request.jwt.claims', '{"sub":"71000000-0000-0000-0000-000000000002"}', true);
select is(public.task_invitation_board('71000000-0000-0000-0000-000000000000'), null,
  'a revoked invitation shows nothing from the next read');
select throws_ok(format('select public.task_invitation_accept(%L)', :'invitation_id'),
  '42501', 'this invitation is not open to the signed-in email', 'a revoked invitation cannot be accepted again');

select * from finish();
rollback;
