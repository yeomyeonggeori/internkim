begin;
create extension if not exists pgtap with schema extensions;
select plan(16);

insert into auth.users (id, email) values
  ('43100000-0000-0000-0000-000000000001', 'current-owner@example.com'),
  ('43100000-0000-0000-0000-000000000002', 'current-peer@example.com');
insert into public.company (id, name, slug, country, locale, timezone, work_locations, rules) values
  ('43100000-0000-0000-0000-000000000000', 'Current Sample', 'current-sample', 'US', 'en', 'America/New_York',
   '[{"name":"Office","color":"blue"}]', '{"teamViewVisibleToAll":false,"attendanceLeavePolicy":{"leaveTypes":[{"id":"retired","name":"Historical leave","isActive":false}]}}');
insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('43100000-0000-0000-0000-000000000011', '43100000-0000-0000-0000-000000000000', 'current-owner@example.com', '43100000-0000-0000-0000-000000000001', 'active', false),
  ('43100000-0000-0000-0000-000000000012', '43100000-0000-0000-0000-000000000000', 'current-peer@example.com', '43100000-0000-0000-0000-000000000002', 'active', false);

select ok(not (select prosecdef from pg_proc where oid='public.attendance_current()'::regprocedure), 'current state runs under caller RLS');
select ok(has_function_privilege('authenticated','public.attendance_current()','execute'), 'authenticated members can call');
select ok(not has_function_privilege('anon','public.attendance_current()','execute'), 'anonymous role cannot call');
select ok(not has_function_privilege('service_role','public.attendance_current()','execute'), 'service role is not an alternate read path');

insert into public.attendance (id, member_id, kind, occurred_at, location) values
  ('43100000-0000-0000-0000-000000000021', '43100000-0000-0000-0000-000000000011', 'clock_in', now()-interval '60 days', 'Office'),
  ('43100000-0000-0000-0000-000000000022', '43100000-0000-0000-0000-000000000012', 'clock_in', now()-interval '1 hour', 'Office');

set local role authenticated;
select set_config('request.jwt.claim.sub','43100000-0000-0000-0000-000000000001',true);
select is(public.attendance_current()->>'memberID','43100000-0000-0000-0000-000000000011','identity is resolved from the caller');
select is(public.attendance_current()->>'timeZone','America/New_York','company time zone is authoritative');
select is(jsonb_array_length(public.attendance_current()->'todayEvents'),0,'a colleague today is not returned');
select is(public.attendance_current()->'latestEvent'->>'id','43100000-0000-0000-0000-000000000021','old unclosed clock before the month remains visible');
select is(public.attendance_current()->'authorization'->>'teamViewVisibleToAll','false','team visibility is reported without granting it');
reset role;

insert into public.leave (member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at)
values ('43100000-0000-0000-0000-000000000011','retired',true,false,-0.5,'approved',now()-interval '1 hour',now()+interval '1 hour');
set local role authenticated;
select is(public.attendance_current()->'activeLeave'->>'kindName','Historical leave','active leave retains inactive historical type labels');
select is((public.attendance_current()->'activeLeave'->>'days')::numeric,0.5,'partial leave deduction keeps its positive display amount');
reset role;

insert into public.member (company_id, email, name, status)
select '43100000-0000-0000-0000-000000000000', 'scale-'||number||'@example.com', 'Sample '||number, 'active'
from generate_series(1,10000) as number;
set local role authenticated;
select is(jsonb_array_length(public.attendance_current()->'todayEvents'),0,'ten thousand colleagues do not grow own event response');
reset role;
update public.member set email=null where id='43100000-0000-0000-0000-000000000011';
set local role authenticated;
select is(public.attendance_current()->>'email','','a bound account with nullable directory email still has a valid snapshot');
reset role;
update public.member set status='pending', is_admin=true where id='43100000-0000-0000-0000-000000000011';
set local role authenticated;
select is(public.attendance_current()->'authorization'->>'isAdmin','false','a pending administrator has no active admin authority');
reset role;
update public.member set status='departed' where id='43100000-0000-0000-0000-000000000011';
set local role authenticated;
select throws_ok($$select public.attendance_current()$$,'42501','only a current signed-in member reads their attendance','a departed member cannot read old state');
select set_config('request.jwt.claim.sub','43100000-0000-0000-0000-000000000099',true);
select throws_ok($$select public.attendance_current()$$,'42501','only a current signed-in member reads their attendance','a nonmember cannot select a company');
select * from finish();
rollback;
