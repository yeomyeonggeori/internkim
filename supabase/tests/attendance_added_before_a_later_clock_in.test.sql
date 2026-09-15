begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

insert into auth.users (id, email) values
	('42000000-0000-0000-0000-000000000001', 'later-boss@example.test');

insert into public.company (
	id, name, slug, country, locale, timezone, work_locations, work_hours, minimum_daily_minutes
) values (
	'42000000-0000-0000-0000-000000000000', 'Sample Later', 'later-a', 'KR', 'ko', 'Asia/Seoul',
	'[{"name":"Office"}]', '[[[],[],[],[],[],null,null]]', 480
);

insert into public.member (id, company_id, email, name, user_id, status, is_admin) values (
	'42000000-0000-0000-0000-000000000011', '42000000-0000-0000-0000-000000000000',
	'later-boss@example.test', '박예시', '42000000-0000-0000-0000-000000000001', 'active', true
);

insert into public.attendance (member_id, kind, location, occurred_at) values
	('42000000-0000-0000-0000-000000000011', 'clock_in', 'Office', '2026-08-24 09:00:00+09'),
	('42000000-0000-0000-0000-000000000011', 'clock_out', null, '2026-08-24 18:00:00+09');

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '42000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_add(
		'42000000-0000-0000-0000-000000000011', 'clock_in', '2026-08-10', '09:00'::time, 'Office', null
	);
	assert answer ->> 'status' = 'added', 'the clock-in is written';
end $$;$block$, 'a clock-in is added on a day before a later clock-in at the same place');

select lives_ok($block$do $$
declare
	answer jsonb;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '42000000-0000-0000-0000-000000000001', true);
	answer := public.attendance_add(
		'42000000-0000-0000-0000-000000000011', 'clock_out', '2026-08-10', '18:00'::time, null, null
	);
	assert answer ->> 'status' = 'added', 'the clock-out is written';
end $$;$block$, 'the clock-out that closes that day follows it');

select is(
	(
		select array_agg(kind::text order by occurred_at)
		from public.attendance
		where member_id = '42000000-0000-0000-0000-000000000011' and deleted_at is null
	),
	array['clock_in', 'clock_out', 'clock_in', 'clock_out'],
	'the added day stands before the later one'
);

select throws_ok($block$do $$
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '42000000-0000-0000-0000-000000000001', true);
	perform public.attendance_add(
		'42000000-0000-0000-0000-000000000011', 'clock_in', '2026-08-10', '10:00'::time, 'Office', null
	);
end $$;$block$, '23514', 'already clocked in at Office', 'a clock-in right after a clock-in at the same place is still refused');

select * from finish();
rollback;
