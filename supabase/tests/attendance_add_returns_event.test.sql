begin;
create extension if not exists pgtap with schema extensions;
select plan(11);

insert into auth.users (id, email) values
	('43000000-0000-0000-0000-000000000001', 'clock-owner@example.test'),
	('43000000-0000-0000-0000-000000000002', 'clock-outsider@example.test');

insert into public.company (
	id, name, slug, country, locale, timezone, work_locations, work_hours, minimum_daily_minutes
) values
	('43000000-0000-0000-0000-000000000000', 'Sample Clock', 'sample-clock', 'KR', 'ko', 'Asia/Seoul',
	 '[{"name":"Office"},{"name":"Branch"}]', '[[[],[],[],[],[],null,null]]', 480);

insert into public.member (id, company_id, email, user_id, status) values
	('43000000-0000-0000-0000-000000000011', '43000000-0000-0000-0000-000000000000',
	 'clock-owner@example.test', '43000000-0000-0000-0000-000000000001', 'active'),
	('43000000-0000-0000-0000-000000000012', '43000000-0000-0000-0000-000000000000',
	 'clock-outsider@example.test', '43000000-0000-0000-0000-000000000002', 'active');

set local role authenticated;
select set_config('request.jwt.claim.sub', '43000000-0000-0000-0000-000000000001', true);

select lives_ok($block$do $$
declare
	answer jsonb;
	event_id uuid;
	event public.attendance;
begin
	answer := public.attendance_add(null, 'clock_in', null, null, null, null);
	assert answer ->> 'status' = 'added', 'an authenticated member can clock in without a target';
	event_id := (answer ->> 'eventID')::uuid;
	select * into event from public.attendance where id = event_id;
	assert event.member_id = '43000000-0000-0000-0000-000000000011', 'the event belongs to the caller';
	assert event.location = 'Office', 'the event contains the resolved default location';
	assert (answer -> 'event' ->> 'id')::uuid = event.id, 'the answer returns the saved event id';
	assert answer -> 'event' ->> 'personID' = event.member_id::text, 'the answer returns the saved person';
	assert answer -> 'event' ->> 'kind' = event.kind::text, 'the answer returns the saved kind';
	assert (answer -> 'event' ->> 'occurredAt')::timestamptz = event.occurred_at, 'the answer returns the saved timestamp';
	assert answer -> 'event' ->> 'location' = event.location, 'the answer returns the saved location';
end $$;$block$, 'a live clock-in returns its exact saved event');

reset role;
update public.attendance
	set occurred_at = occurred_at - interval '2 minutes', edit_reason = 'clocked in earlier'
	where member_id = '43000000-0000-0000-0000-000000000011';
set local role authenticated;
select set_config('request.jwt.claim.sub', '43000000-0000-0000-0000-000000000001', true);

select lives_ok($block$do $$
declare answer jsonb;
begin
	answer := public.attendance_add(null, 'clock_out', null, null, null, null);
	assert answer ->> 'status' = 'added', 'an authenticated member can clock out without a target';
	assert answer -> 'event' ->> 'location' is null, 'a clock-out event has no location';
end $$;$block$, 'a live clock-out returns its exact saved event');

select throws_ok(
	$$select public.attendance_add(null, 'clock_out', null, null, null, null)$$,
	'23514', 'cannot clock out without being clocked in',
	'a clock-out without an open shift is rejected'
);

select lives_ok($block$do $$
declare refused boolean := false;
begin
	begin
		perform public.attendance_add('43000000-0000-0000-0000-000000000012', 'clock_in', null, null, 'Office', null);
	exception when insufficient_privilege then
		refused := true;
	end;
	assert refused, 'a member cannot write another member attendance';
end $$;$block$, 'a foreign member target is rejected');

select lives_ok($block$do $$
declare refused boolean := false;
begin
	reset role;
	set local role anon;
	perform set_config('request.jwt.claim.sub', '', true);
	begin
		perform public.attendance_add(null, 'clock_in', null, null, null, null);
	exception when insufficient_privilege then
		refused := true;
	end;
	assert refused, 'an unauthenticated caller cannot clock in';
	set local role authenticated;
end $$;$block$, 'an unauthenticated caller is rejected');

set local role authenticated;
select set_config('request.jwt.claim.sub', '43000000-0000-0000-0000-000000000001', true);
select lives_ok($$select public.attendance_add(null, 'clock_in', null, null, 'Branch', null)$$,
	'a member can clock in again at a different location');
select throws_ok(
	$$select public.attendance_add(null, 'clock_in', null, null, 'Branch', null)$$,
	'23514', 'already clocked in at Branch',
	'a duplicate live clock-in is rejected');
select throws_ok(
	$$select public.attendance_add(null, 'clock_in', null, null, 'Unknown', null)$$,
	'23514', 'location Unknown is not one of the registered work locations',
	'an invalid live location is rejected');

select lives_ok($$select public.attendance_add(null, 'clock_out', null, null, null, null)$$,
	'a live clock-out closes the second shift');
select lives_ok($$select public.attendance_add(null, 'clock_in', current_date - 1, '09:00', 'Office', null)$$,
	'a historical clock-in keeps the existing hand-written path');
select is(
	(select occurred_at from public.attendance where member_id = '43000000-0000-0000-0000-000000000011'
	 and occurred_at = ((current_date - 1) + time '09:00') at time zone 'Asia/Seoul'),
	((current_date - 1) + time '09:00') at time zone 'Asia/Seoul',
	'an explicit historical timestamp is preserved'
);

select * from finish();
rollback;
