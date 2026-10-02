begin;
create extension if not exists pgtap with schema extensions;
select plan(32);

insert into auth.users (id, email) values
  ('63000000-0000-0000-0000-000000000001', 'link-admin@example.com'),
  ('63000000-0000-0000-0000-000000000002', 'link-employee@example.com');
insert into public.company (id, name, slug, country, locale, timezone)
  values ('63000000-0000-0000-0000-000000000010', 'Link Sample', 'link-sample', 'KR', 'ko', 'Asia/Seoul');
insert into public.member (id, company_id, user_id, email, status, is_admin) values
  ('63000000-0000-0000-0000-000000000011', '63000000-0000-0000-0000-000000000010', '63000000-0000-0000-0000-000000000001', 'link-admin@example.com', 'active', true),
  ('63000000-0000-0000-0000-000000000012', '63000000-0000-0000-0000-000000000010', '63000000-0000-0000-0000-000000000002', 'link-employee@example.com', 'active', false);
insert into public.company_document (company_id, document_type, title, category_code) values
  ('63000000-0000-0000-0000-000000000010', 'report', 'Statements', 'FS'),
  ('63000000-0000-0000-0000-000000000010', 'report', 'Payroll', 'FP'),
  ('63000000-0000-0000-0000-000000000010', 'report', 'Inbox', 'X');

set local role authenticated;
select set_config('request.jwt.claims', '{"sub":"63000000-0000-0000-0000-000000000001"}', true);
select public.data_room_role_set('63000000-0000-0000-0000-000000000010', 'statements', 'Statements', array['FS']);
select public.data_room_role_set('63000000-0000-0000-0000-000000000010', 'creator-statements', 'Creator statements', array['FS']);
select lives_ok($$select public.data_room_member_roles_set('63000000-0000-0000-0000-000000000010', '63000000-0000-0000-0000-000000000012', array['creator-statements'])$$, 'administrators assign employee roles');
select is((select count(*) from public.data_room_share where member_id = '63000000-0000-0000-0000-000000000012' and revoked_at is null), 1::bigint, 'assignments replace previous direct grants');
select throws_ok($$select public.data_room_member_roles_set('63000000-0000-0000-0000-000000000010', '63000000-0000-0000-0000-000000000012', array['unknown'])$$,
  '22023', 'existing company roles required', 'unknown roles fail atomically');
select is((select role_code from public.data_room_share where member_id = '63000000-0000-0000-0000-000000000012' and revoked_at is null), 'creator-statements', 'failed assignments preserve previous access');
select public.data_room_link_create('63000000-0000-0000-0000-000000000010', 'leadership', 'Leadership review', '123456') as leadership_link \gset
select ok(:'leadership_link' is not null, 'leadership is shareable when the creator has its permissions');

select set_config('request.jwt.claims', '{"sub":"63000000-0000-0000-0000-000000000002"}', true);
select throws_ok($$select public.data_room_member_roles_set('63000000-0000-0000-0000-000000000010', '63000000-0000-0000-0000-000000000012', array['leadership'])$$,
  '42501', 'only administrators assign employee reader roles', 'employees cannot grant themselves roles');
select throws_ok($$select public.data_room_link_create('63000000-0000-0000-0000-000000000010', 'accountant', 'Too broad', '123456')$$,
  '42501', 'the reader role exceeds your permissions', 'links cannot exceed the creator scope');
select throws_ok($$select public.data_room_link_create('63000000-0000-0000-0000-000000000010', 'statements', 'Too long', '123456', 169)$$,
  '22023', 'six digit code and supported lifetime required', 'links cannot live more than seven days');
select throws_ok($$select public.data_room_link_create('63000000-0000-0000-0000-000000000010', 'statements', 'Bad code', '12345')$$,
  '22023', 'six digit code and supported lifetime required', 'exactly six digits are required');
select public.data_room_link_create('63000000-0000-0000-0000-000000000010', 'statements', 'Review', '123456') as link_id \gset
select is((select expires_at - created_at from public.data_room_link where id = :'link_id'), interval '3 days', 'default lifetime is three days');
select throws_ok($$select code_hash from public.data_room_link$$, '42501', 'permission denied for table data_room_link', 'employees cannot read password hashes');

reset role;
select ok((select code_hash <> '123456' from public.data_room_link where id = :'link_id'), 'only a password hash is stored');
select is(public.data_room_link_unlock(:'link_id', '123456', 'wrong', repeat('a',64)), null::jsonb, 'the current notice must be acknowledged');
select is(public.data_room_link_unlock(:'link_id', '654321', '1', repeat('a',64)), null::jsonb, 'wrong codes cannot create a session');
select public.data_room_link_unlock(:'link_id', '123456', '1', repeat('a',64)) ->> 'sessionID' as session_id \gset
select public.data_room_link_unlock(:'leadership_link', '123456', '1', repeat('a',64)) ->> 'sessionID' as leadership_session \gset
select ok(:'session_id' is not null, 'correct code and acknowledgment create a guest session');
select is((select count(*) from public.member where user_id = :'session_id'::uuid), 0::bigint, 'guest sessions create no company membership');

set local role authenticated;
select set_config('request.jwt.claims', jsonb_build_object('sub', :'leadership_session')::text, true);
select ok(public.data_room_may_read('63000000-0000-0000-0000-000000000010', 'X'), 'X follows the role and creator permissions in a protected link');
select set_config('request.jwt.claims', jsonb_build_object('sub', :'session_id')::text, true);
select is((select count(*) from public.company_document where company_id = '63000000-0000-0000-0000-000000000010'), 1::bigint, 'guest RLS exposes only the linked role');
select ok(not public.data_room_may_read('63000000-0000-0000-0000-000000000010', 'FS', true), 'a read-only link does not authorize downloads');
select ok(not public.data_room_may_read('63000000-0000-0000-0000-000000000010', 'X'), 'a role without X cannot expose the inbox');
select ok(not public.data_room_may_read('63000000-0000-0000-0000-000000000099', 'FS'), 'sessions cannot cross company boundaries');
select throws_ok(format('select public.data_room_link_revoke(%L)', :'link_id'), '42501', 'only the creator or an administrator revokes this link', 'guests cannot revoke links');

select set_config('request.jwt.claims', '{"sub":"63000000-0000-0000-0000-000000000001"}', true);
select public.data_room_role_set('63000000-0000-0000-0000-000000000010', 'statements', 'Statements', array['FS', 'FP']);
select set_config('request.jwt.claims', jsonb_build_object('sub', :'session_id')::text, true);
select ok(not public.data_room_may_read('63000000-0000-0000-0000-000000000010', 'FP'), 'role expansion cannot exceed the creator permissions');
select set_config('request.jwt.claims', '{"sub":"63000000-0000-0000-0000-000000000001"}', true);
select public.data_room_member_roles_set('63000000-0000-0000-0000-000000000010', '63000000-0000-0000-0000-000000000012', array[]::text[]);
select set_config('request.jwt.claims', jsonb_build_object('sub', :'session_id')::text, true);
select ok(not public.data_room_may_read('63000000-0000-0000-0000-000000000010', 'FS'), 'creator permission removal immediately restricts existing sessions');

select set_config('request.jwt.claims', '{"sub":"63000000-0000-0000-0000-000000000002"}', true);
select lives_ok(format('select public.data_room_link_revoke(%L)', :'link_id'), 'creators can revoke their links after losing reader access');
select set_config('request.jwt.claims', jsonb_build_object('sub', :'session_id')::text, true);
select ok(not public.data_room_may_read('63000000-0000-0000-0000-000000000010', 'FS'), 'revocation blocks an already open session');
reset role;
select is(public.data_room_link_unlock(:'link_id', '123456', '1', repeat('a',64)), null::jsonb, 'revoked links cannot be reopened');
update public.data_room_link set revoked_at = null where id = :'link_id';
update public.data_room_link_attempt set attempt_count = 10 where link_id = :'link_id' and fingerprint = repeat('a',64);
select is(public.data_room_link_unlock(:'link_id', '123456', '1', repeat('a',64)), null::jsonb, 'attempt limits persist in the database');
select ok(public.data_room_link_unlock(:'link_id', '123456', '1', repeat('b',64)) is not null, 'one network address cannot exhaust another address limit');
update public.data_room_link set attempt_count = 100 where id = :'link_id';
select is(public.data_room_link_unlock(:'link_id', '123456', '1', repeat('c',64)), null::jsonb, 'the shared limit bounds distributed guessing');
update public.data_room_link set created_at = now() - interval '2 days', expires_at = now() - interval '1 minute', attempt_count = 0 where id = :'link_id';
select is(public.data_room_link_unlock(:'link_id', '123456', '1', repeat('c',64)), null::jsonb, 'expired links cannot issue sessions');
set local role authenticated;
select set_config('request.jwt.claims', jsonb_build_object('sub', :'session_id')::text, true);
select ok(not public.data_room_may_read('63000000-0000-0000-0000-000000000010', 'FS'), 'expired links block existing sessions');

select * from finish();
rollback;
