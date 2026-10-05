begin;
create extension if not exists pgtap with schema extensions;
select plan(29);

insert into auth.users (id, email) values
  ('43200000-0000-0000-0000-000000000001', 'team-owner@example.com'),
  ('43200000-0000-0000-0000-000000000002', 'team-peer@example.com');
insert into public.company (id, name, slug, country, locale, timezone, work_locations, rules) values
  ('43200000-0000-0000-0000-000000000000', 'Team Sample', 'team-sample', 'US', 'en', 'America/New_York',
   '[{"name":"Office","color":"blue"}]', '{"teamViewVisibleToAll":true}');
insert into public.team (id, company_id, name, position) values
  ('43200000-0000-0000-0000-000000000010', '43200000-0000-0000-0000-000000000000', 'Alpha', 0),
  ('43200000-0000-0000-0000-000000000020', '43200000-0000-0000-0000-000000000000', 'Beta', 1);
insert into public.member (id, company_id, email, user_id, status, is_admin, name, team_id) values
  ('43200000-0000-0000-0000-000000000011', '43200000-0000-0000-0000-000000000000', 'team-owner@example.com', '43200000-0000-0000-0000-000000000001', 'active', true, 'Owner Sample', '43200000-0000-0000-0000-000000000010'),
  ('43200000-0000-0000-0000-000000000012', '43200000-0000-0000-0000-000000000000', 'team-peer@example.com', '43200000-0000-0000-0000-000000000002', 'active', false, 'Peer Sample', '43200000-0000-0000-0000-000000000010'),
  ('43200000-0000-0000-0000-000000000013', '43200000-0000-0000-0000-000000000000', 'unassigned@example.com', null, 'active', false, 'Unassigned Sample', null);

select ok(not (select prosecdef from pg_proc where oid='public.attendance_team_page(text,integer,integer,text,integer,integer,text,text)'::regprocedure), 'paged read uses caller RLS');
select ok(has_function_privilege('authenticated','public.attendance_team_page(text,integer,integer,text,integer,integer,text,text)','execute'), 'authenticated may read');
select ok(not has_function_privilege('anon','public.attendance_team_page(text,integer,integer,text,integer,integer,text,text)','execute'), 'anonymous cannot call');
select ok(not has_function_privilege('service_role','public.attendance_team_page(text,integer,integer,text,integer,integer,text,text)','execute'), 'service role is not an alternate read path');

insert into public.attendance (member_id, kind, occurred_at, location) values
  ('43200000-0000-0000-0000-000000000011', 'clock_in', now()-interval '30 minutes', 'Office'),
  ('43200000-0000-0000-0000-000000000012', 'clock_in', now()-interval '25 minutes', 'Office'),
  ('43200000-0000-0000-0000-000000000012', 'clock_out', now()-interval '20 minutes', null),
  ('43200000-0000-0000-0000-000000000011', 'clock_out', now()-interval '15 minutes', null),
  ('43200000-0000-0000-0000-000000000011', 'clock_in', now()-interval '10 minutes', 'Office'),
  ('43200000-0000-0000-0000-000000000012', 'clock_in', now()-interval '7 minutes', 'Office'),
  ('43200000-0000-0000-0000-000000000012', 'clock_out', now()-interval '5 minutes', null),
  ('43200000-0000-0000-0000-000000000013', 'clock_in', now()-interval '1 day', 'Office');

set local role authenticated;
select set_config('request.jwt.claim.sub','43200000-0000-0000-0000-000000000001',true);
select is((public.attendance_team_page('teams')->>'teamTotal')::integer,3,'teams include the unassigned group');
select is(jsonb_array_length(public.attendance_team_page('teams',0,1)->'teams'),1,'team page is bounded');
select is(public.attendance_team_page('teams',0,1)->'teams'->0->>'name','Alpha','team order is stable');
select is((public.attendance_team_page('teams',0,1)->'teams'->0->>'working')::integer,1,'working counts use the latest actual event');
select is((public.attendance_team_page('teams',0,1)->'teams'->0->>'done')::integer,1,'clocked-out employees count separately');
select is(public.attendance_team_page('teams',0,1)->'teams'->0->'recentClockOuts'->0->>'name','Peer Sample','recent actor is an actual clock-out');
select is(jsonb_array_length(public.attendance_team_page('teams',0,1)->'teams'->0->'recentClockIns'),2,'recent actor stack contains distinct people');
select is((public.attendance_team_page('teams',0,1)->'teams'->0->'recordedLocations'->0->>'count')::integer,1,'locations count current workers, not clock-in events');
select is((public.attendance_team_page('teams',2,1)->'teams'->0->>'working')::integer,1,'an older open shift is grouped into current working attendance');
select is((public.attendance_team_page('members',0,12,'43200000-0000-0000-0000-000000000010',0,1)->>'memberTotal')::integer,2,'member total precedes page limit');
select is(jsonb_array_length(public.attendance_team_page('members',0,12,'43200000-0000-0000-0000-000000000010',0,1)->'members'),1,'employee page is bounded');
select is((public.attendance_team_page('members',0,12,'43200000-0000-0000-0000-000000000010',0,24,'Peer')->>'memberTotal')::integer,1,'name search runs before paging');
select is((public.attendance_team_page('members',0,12,'43200000-0000-0000-0000-000000000010',0,24,'','Office')->>'memberTotal')::integer,2,'recorded location filter includes clocked-out colleague');
select throws_ok($$select public.attendance_team_page(page_kind=>null)$$,'22023','invalid attendance page','explicit null mode is refused');
select throws_ok($$select public.attendance_team_page(team_limit=>null)$$,'22023','invalid attendance page','explicit null team limit cannot unbound the read');
select throws_ok($$select public.attendance_team_page(member_limit=>null)$$,'22023','invalid attendance page','explicit null member limit cannot unbound the read');
select throws_ok($$select public.attendance_team_page(team_limit=>25)$$,'22023','invalid attendance page','oversize team page is refused');
select throws_ok($$select public.attendance_team_page(member_limit=>49)$$,'22023','invalid attendance page','oversize member page is refused');
select throws_ok($$select public.attendance_team_page(search_text=>null)$$,'22023','invalid attendance page','explicit null filter is refused');
select throws_ok($$select public.attendance_team_page('members',0,12,'43200000-0000-0000-0000-000000000099')$$,'42501','the selected team is outside this company','foreign team cannot be selected');
reset role;

insert into public.member (company_id, email, name, status, team_id)
select '43200000-0000-0000-0000-000000000000',
    'team-scale-'||number||'@example.com', 'Scale '||number, 'active',
    '43200000-0000-0000-0000-000000000010'
from generate_series(1,10000) as number;
set local role authenticated;
select is((public.attendance_team_page('members',0,12,'43200000-0000-0000-0000-000000000010',0,24)->>'memberTotal')::integer,10002,'ten thousand colleagues remain reachable through pages');
select is(jsonb_array_length(public.attendance_team_page('members',0,12,'43200000-0000-0000-0000-000000000010',0,24)->'members'),24,'ten thousand colleagues do not enlarge the employee response');
select ok(octet_length(public.attendance_team_page('teams',0,1)::text) < 20000,'team card response stays compact at ten thousand colleagues');
reset role;

update public.company set rules='{"teamViewVisibleToAll":false}' where id='43200000-0000-0000-0000-000000000000';
set local role authenticated;
select set_config('request.jwt.claim.sub','43200000-0000-0000-0000-000000000002',true);
select throws_ok($$select public.attendance_team_page('teams')$$,'42501','the team attendance view is restricted','nonadmin respects company team visibility');
select set_config('request.jwt.claim.sub','43200000-0000-0000-0000-000000000099',true);
select throws_ok($$select public.attendance_team_page('teams')$$,'42501','only a current signed-in member reads attendance','anonymous account cannot select a company');
select * from finish();
rollback;
