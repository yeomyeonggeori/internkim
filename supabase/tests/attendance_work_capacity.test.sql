begin;
create extension if not exists pgtap with schema extensions;
select plan(13);

insert into auth.users (id, email) values
	('10000000-0000-0000-0000-000000000001', 'capacity-a@example.com'),
	('20000000-0000-0000-0000-000000000001', 'capacity-b@example.com');

insert into public.company (
	id, name, slug, country, locale, timezone, work_hours, minimum_daily_minutes, rules
) values
	(
		'10000000-0000-0000-0000-000000000000', 'Capacity A', 'capacity-a', 'KR', 'ko', 'Asia/Seoul',
		'[[[],[],[],[],[],null,null]]', 480, '{"approvals":{"required":true}}'
	),
	(
		'20000000-0000-0000-0000-000000000000', 'Capacity B', 'capacity-b', 'US', 'en-US', 'America/New_York',
		'[[[],[],[],[],[],null,null]]', 420, '{"branding":{"accent":"blue"}}'
	);

insert into public.member (id, company_id, email, user_id, status) values
	(
		'10000000-0000-0000-0000-000000000011', '10000000-0000-0000-0000-000000000000',
		'capacity-a@example.com', '10000000-0000-0000-0000-000000000001', 'active'
	),
	(
		'10000000-0000-0000-0000-000000000012', '10000000-0000-0000-0000-000000000000',
		'capacity-a-colleague@example.com', null, 'active'
	),
	(
		'20000000-0000-0000-0000-000000000011', '20000000-0000-0000-0000-000000000000',
		'capacity-b@example.com', '20000000-0000-0000-0000-000000000001', 'active'
	);

select hasnt_column(
	'public',
	'company',
	'attendance_work_mode',
	'attendance work mode is not stored in a company column'
);

select lives_ok(
	$$select public.save_attendance_calendar(
		'10000000-0000-0000-0000-000000000000'::uuid,
		'[{"date":"2027-01-01","workMode":"fixed","workingDate":false}]'::jsonb
	)$$,
	'attendance calendar persistence accepts a tenant-scoped projection'
);

select is(
	(select rules -> 'approvals' from public.company where id = '10000000-0000-0000-0000-000000000000'),
	'{"required": true}'::jsonb,
	'attendance calendar persistence preserves unrelated company rules'
);

select is(
	(select rules -> 'attendanceCalendar' from public.company where id = '10000000-0000-0000-0000-000000000000'),
	'[{"date":"2027-01-01","workMode":"fixed","workingDate":false}]'::jsonb,
	'attendance calendar persistence replaces only the projected calendar'
);

select lives_ok(
	$$select public.save_attendance_calendar(
		'10000000-0000-0000-0000-000000000000'::uuid,
		'[{"date":"2027-02-02","workMode":"autonomous","workingDate":true},{"date":"2027-02-01","workMode":"fixed","workingDate":false},{"date":"2027-02-01","workMode":"flexible","workingDate":true}]'::jsonb
	)$$,
	'attendance calendar persistence accepts a consecutive monthly projection'
);

select is(
	(select rules -> 'attendanceCalendar' from public.company where id = '10000000-0000-0000-0000-000000000000'),
	'[{"date":"2027-01-01","workMode":"fixed","workingDate":false},{"date":"2027-02-01","workMode":"flexible","workingDate":true},{"date":"2027-02-02","workMode":"autonomous","workingDate":true}]'::jsonb,
	'attendance calendar persistence retains prior dates and sorts the unique merged projection'
);

select lives_ok(
	$$select public.save_attendance_work_policy(
		'10000000-0000-0000-0000-000000000000'::uuid,
		'{"workMode":"fixed","workingWeekdays":[1,2,3,4,5],"dailyTargetMinutes":480,"nightStartTime":"22:00","nightEndTime":"06:00"}'::jsonb
	)$$,
	'attendance work policy persistence accepts one current JSON policy'
);

select is(
	(select rules -> 'approvals' from public.company where id = '10000000-0000-0000-0000-000000000000'),
	'{"required": true}'::jsonb,
	'attendance work policy persistence preserves unrelated company rules'
);

select throws_ok(
	$$select public.save_attendance_work_policy(
		'10000000-0000-0000-0000-000000000000'::uuid,
		'[]'::jsonb
	)$$,
	'23514',
	'attendance work policy must be an object',
	'attendance work policy persistence rejects a non-object value'
);

select is(
	(select rules from public.company where id = '20000000-0000-0000-0000-000000000000'),
	'{"branding":{"accent":"blue"}}'::jsonb,
	'attendance calendar persistence does not update another tenant'
);

set local role authenticated;
select set_config('request.jwt.claim.sub', '10000000-0000-0000-0000-000000000001', true);

select is(
	(select count(*) from public.attendance_work_policies()),
	2::bigint,
	'attendance work policies resolve every visible colleague through member fallback functions'
);

select is(
	(
		select work_calendar
		from public.attendance_work_policies()
		where member_id = '10000000-0000-0000-0000-000000000011'
	),
	'[{"date":"2027-01-01","workMode":"fixed","workingDate":false},{"date":"2027-02-01","workMode":"flexible","workingDate":true},{"date":"2027-02-02","workMode":"autonomous","workingDate":true}]'::jsonb,
	'attendance work policies expose the stored date-level projection'
);

select is(
	(
		select jsonb_build_object('workMode', work_mode, 'workPolicy', work_policy)
		from public.attendance_work_policies()
		where member_id = '10000000-0000-0000-0000-000000000011'
	),
	'{"workMode":"fixed","workPolicy":{"workMode":"fixed","workingWeekdays":[1,2,3,4,5],"dailyTargetMinutes":480,"nightStartTime":"22:00","nightEndTime":"06:00"}}'::jsonb,
	'attendance work policies use and expose the single current policy'
);

select * from finish();
rollback;
