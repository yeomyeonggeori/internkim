begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

insert into auth.users (id, email) values
	('43000000-0000-0000-0000-000000000001', 'removed-boss@example.test');

insert into public.company (
	id, name, slug, country, locale, timezone, work_locations, work_hours, minimum_daily_minutes
) values (
	'43000000-0000-0000-0000-000000000000', 'Sample Removed', 'removed-a', 'KR', 'ko', 'Asia/Seoul',
	'[{"name":"Office"}]', '[[[],[],[],[],[],null,null]]', 480
);

insert into public.member (id, company_id, email, name, user_id, status, is_admin) values (
	'43000000-0000-0000-0000-000000000011', '43000000-0000-0000-0000-000000000000',
	'removed-boss@example.test', '박예시', '43000000-0000-0000-0000-000000000001', 'active', true
);

insert into public.attendance (id, member_id, kind, location, occurred_at) values
	('43000000-0000-0000-0000-000000000101', '43000000-0000-0000-0000-000000000011', 'clock_in', 'Office', '2026-08-10 09:00:00+09'),
	('43000000-0000-0000-0000-000000000102', '43000000-0000-0000-0000-000000000011', 'clock_out', null, '2026-08-10 18:00:00+09');

update public.attendance set deleted_at = now()
	where id in ('43000000-0000-0000-0000-000000000101', '43000000-0000-0000-0000-000000000102');

insert into public.attendance (id, member_id, kind, location, occurred_at) values
	('43000000-0000-0000-0000-000000000103', '43000000-0000-0000-0000-000000000011', 'clock_in', 'Office', '2026-08-10 09:30:00+09'),
	('43000000-0000-0000-0000-000000000104', '43000000-0000-0000-0000-000000000011', 'clock_out', null, '2026-08-10 18:30:00+09'),
	('43000000-0000-0000-0000-000000000105', '43000000-0000-0000-0000-000000000011', 'clock_in', 'Office', '2026-08-11 09:00:00+09'),
	('43000000-0000-0000-0000-000000000106', '43000000-0000-0000-0000-000000000011', 'clock_out', null, '2026-08-11 18:00:00+09');

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '43000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_correct(
		jsonb_build_array(jsonb_build_object(
			'event_id', '43000000-0000-0000-0000-000000000103', 'local_date', '2026-08-10', 'local_time', '08:50'
		)),
		null
	);
	assert answer ->> 'status' = 'corrected', 'the correction is written';
end $$;$block$, 'a record is moved past the time of a removed one');

select is(
	(select occurred_at from public.attendance where id = '43000000-0000-0000-0000-000000000103'),
	'2026-08-10 08:50:00+09'::timestamptz,
	'the moved record holds its new time'
);

reset role;

select throws_ok($block$
	update public.attendance set occurred_at = '2026-08-10 19:00:00+09'
		where id = '43000000-0000-0000-0000-000000000103'
$block$, '23514', 'attendance correction cannot reorder events', 'moving a record past a live one is still refused');

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '43000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_remove('43000000-0000-0000-0000-000000000104', null);
	assert answer ->> 'status' = 'removed', 'the removal is written';
end $$;$block$, 'a record is still removed while removed ones sit beside it');

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '43000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_correct(
		jsonb_build_array(jsonb_build_object(
			'event_id', '43000000-0000-0000-0000-000000000106', 'local_date', '2026-08-11', 'local_time', '17:00'
		)),
		null
	);
	assert answer ->> 'status' = 'corrected', 'a correction with nothing removed in between is written';
end $$;$block$, 'a correction that passes no removed record is unchanged');

select * from finish();
rollback;
