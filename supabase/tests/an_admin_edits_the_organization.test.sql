begin;
create extension if not exists pgtap with schema extensions;
select plan(8);

insert into auth.users (id, email) values
	('41000000-0000-0000-0000-000000000001', 'organization-admin@example.com'),
	('41000000-0000-0000-0000-000000000002', 'organization-member@example.com'),
	('42000000-0000-0000-0000-000000000001', 'organization-outsider@example.com');

insert into public.company (id, name, slug, country, locale, timezone) values
	('41000000-0000-0000-0000-000000000000', 'Sample Organization A', 'organization-a', 'KR', 'ko', 'Asia/Seoul'),
	('42000000-0000-0000-0000-000000000000', 'Sample Organization B', 'organization-b', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, name, email, user_id, status, is_admin) values
	(
		'41000000-0000-0000-0000-000000000011', '41000000-0000-0000-0000-000000000000',
		'이샘플', 'organization-admin@example.com', '41000000-0000-0000-0000-000000000001', 'active', true
	),
	(
		'41000000-0000-0000-0000-000000000012', '41000000-0000-0000-0000-000000000000',
		'박예시', 'organization-member@example.com', '41000000-0000-0000-0000-000000000002', 'active', false
	),
	(
		'41000000-0000-0000-0000-000000000013', '41000000-0000-0000-0000-000000000000',
		'최견본', 'organization-colleague@example.com', null, 'active', false
	),
	(
		'42000000-0000-0000-0000-000000000011', '42000000-0000-0000-0000-000000000000',
		'이샘플', 'organization-outsider@example.com', '42000000-0000-0000-0000-000000000001', 'active', true
	);

insert into public.team (id, company_id, name, position) values
	('41000000-0000-0000-0000-0000000000a1', '41000000-0000-0000-0000-000000000000', '개발팀', 0),
	('42000000-0000-0000-0000-0000000000b1', '42000000-0000-0000-0000-000000000000', '남의 조직', 0);

select lives_ok($block$do $$
declare
	made uuid;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);

	made := public.team_add('플랫폼', '41000000-0000-0000-0000-0000000000a1', 1);

	assert (
		select count(*) from public.team where company_id = '41000000-0000-0000-0000-000000000000'
	) = 2, 'an administrator adds one organization at a time';
	assert (
		select position from public.team where id = made
	) = 1, 'an organization takes the position it was given';
	assert (
		select parent_team_id from public.team where id = made
	) = '41000000-0000-0000-0000-0000000000a1', 'a child organization keeps the parent it was given';

	perform public.team_update(made, '{"name": "플랫폼실", "position": 2}'::jsonb);

	assert (
		select name = '플랫폼실' and position = 2
			and parent_team_id = '41000000-0000-0000-0000-0000000000a1'
		from public.team where id = made
	), 'a change leaves the fields it did not name alone';
	assert (
		select name from public.team where id = '41000000-0000-0000-0000-0000000000a1'
	) = '개발팀', 'a change reaches only the organization it named';
end $$;$block$, 'an administrator adds and changes one organization');

select lives_ok($block$do $$
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);

	perform public.person_set(
		'41000000-0000-0000-0000-000000000012',
		'{"name": "박예시", "isAdmin": false}'::jsonb,
		$organization${
			"jobTitle": "  프론트엔드 개발자  ",
			"groupID": "41000000-0000-0000-0000-0000000000a1",
			"hireDate": "2026-03-12",
			"phoneNumber": "010-0000-0000",
			"supervisorID": "41000000-0000-0000-0000-000000000011"
		}$organization$::jsonb
	);

	assert (
		select job_title = '프론트엔드 개발자'
			and team_id = '41000000-0000-0000-0000-0000000000a1'
			and joined_at = date '2026-03-12'
			and phone_number = '010-0000-0000'
			and supervisor_id = '41000000-0000-0000-0000-000000000011'
		from public.member where id = '41000000-0000-0000-0000-000000000012'
	), 'an administrator saves every organization attribute of a colleague';

	perform public.person_set(
		'41000000-0000-0000-0000-000000000012',
		'{}'::jsonb,
		$cleared${
			"jobTitle": "",
			"groupID": "",
			"hireDate": "",
			"phoneNumber": "",
			"supervisorID": ""
		}$cleared$::jsonb
	);

	assert (
		select job_title is null and team_id is null and joined_at is null
			and phone_number is null and supervisor_id is null
		from public.member where id = '41000000-0000-0000-0000-000000000012'
	), 'an emptied field clears the attribute instead of storing an empty string';
	assert (
		select name from public.member where id = '41000000-0000-0000-0000-000000000012'
	) = '박예시', 'a write to one home leaves the other home alone';
end $$;$block$, 'an administrator saves a member organization profile');

select lives_ok($block$do $$
declare
	own_phone_saved boolean := false;
	job_title_refused text := '';
	colleague_refused text := '';
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000002', true);

	perform public.person_set(
		'41000000-0000-0000-0000-000000000012',
		'{}'::jsonb,
		'{"phoneNumber": "010-1111-2222", "hireDate": "2026-04-01"}'::jsonb
	);
	own_phone_saved := (
		select phone_number = '010-1111-2222' and joined_at = date '2026-04-01'
		from public.member where id = '41000000-0000-0000-0000-000000000012'
	);
	assert own_phone_saved, 'a person corrects their own phone number and start date';

	begin
		perform public.person_set(
			'41000000-0000-0000-0000-000000000012',
			'{}'::jsonb,
			'{"jobTitle": "대표"}'::jsonb
		);
	exception when insufficient_privilege then
		job_title_refused := sqlerrm;
	end;
	assert job_title_refused like '%jobTitle%',
		'a field a person may not write is refused by name';

	begin
		perform public.person_set(
			'41000000-0000-0000-0000-000000000013',
			'{}'::jsonb,
			'{"phoneNumber": "010-3333-4444"}'::jsonb
		);
	exception when insufficient_privilege then
		colleague_refused := sqlerrm;
	end;
	assert colleague_refused like '%phoneNumber%',
		'a colleague''s field is refused by name too';
end $$;$block$, 'a person corrects their own profile and nothing else');

select lives_ok($block$do $$
declare
	admin_raise_blocked boolean := false;
	departure_blocked boolean := false;
	team_add_blocked boolean := false;
	team_update_blocked boolean := false;
	team_delete_blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000002', true);

	begin
		perform public.person_set(
			'41000000-0000-0000-0000-000000000012', '{"isAdmin": true}'::jsonb, '{}'::jsonb
		);
	exception when insufficient_privilege then
		admin_raise_blocked := true;
	end;
	assert admin_raise_blocked, 'a member cannot make themselves an administrator';

	begin
		perform public.person_set(
			'41000000-0000-0000-0000-000000000013', '{}'::jsonb, '{"employmentStatus": "departed"}'::jsonb
		);
	exception when insufficient_privilege then
		departure_blocked := true;
	end;
	assert departure_blocked, 'a member cannot mark a colleague departed';

	begin
		perform public.team_add('영업팀', null, null);
	exception when insufficient_privilege then
		team_add_blocked := true;
	end;
	assert team_add_blocked, 'a member who is not an administrator cannot add an organization';

	begin
		perform public.team_update('41000000-0000-0000-0000-0000000000a1', '{"name": "빼앗은 조직"}'::jsonb);
	exception when insufficient_privilege then
		team_update_blocked := true;
	end;
	assert team_update_blocked, 'a member who is not an administrator cannot rename an organization';

	begin
		perform public.team_delete('41000000-0000-0000-0000-0000000000a1');
	exception when insufficient_privilege then
		team_delete_blocked := true;
	end;
	assert team_delete_blocked, 'a member who is not an administrator cannot remove an organization';
end $$;$block$, 'only an administrator edits the organization');

select lives_ok($block$do $$
declare
	foreign_member_blocked boolean := false;
	foreign_team_blocked boolean := false;
	foreign_parent_blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '42000000-0000-0000-0000-000000000001', true);

	begin
		perform public.person_set(
			'41000000-0000-0000-0000-000000000013', '{}'::jsonb, '{"jobTitle": "대표"}'::jsonb
		);
	exception when raise_exception then
		foreign_member_blocked := true;
	end;
	assert foreign_member_blocked, 'an administrator cannot edit a member of another company';

	begin
		perform public.team_update('41000000-0000-0000-0000-0000000000a1', '{"name": "빼앗은 조직"}'::jsonb);
	exception when raise_exception then
		foreign_team_blocked := true;
	end;
	assert foreign_team_blocked, 'an administrator cannot claim an organization of another company';

	begin
		perform public.team_add('바깥팀', '41000000-0000-0000-0000-0000000000a1', null);
	exception when raise_exception then
		foreign_parent_blocked := true;
	end;
	assert foreign_parent_blocked, 'an organization of another company cannot become a parent';
end $$;$block$, 'another company cannot reach this organization');

select lives_ok($block$do $$
declare
	child uuid;
	supervisor_loop_blocked boolean := false;
	team_loop_blocked boolean := false;
	bad_status_blocked boolean := false;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);

	assert (
		select name from public.team where id = '41000000-0000-0000-0000-0000000000a1'
	) = '개발팀', 'a write refused from another company left the organization untouched';

	perform public.person_set(
		'41000000-0000-0000-0000-000000000011',
		'{}'::jsonb,
		'{"supervisorID": "41000000-0000-0000-0000-000000000013"}'::jsonb
	);
	begin
		perform public.person_set(
			'41000000-0000-0000-0000-000000000013',
			'{}'::jsonb,
			'{"supervisorID": "41000000-0000-0000-0000-000000000011"}'::jsonb
		);
	exception when raise_exception then
		supervisor_loop_blocked := true;
	end;
	assert supervisor_loop_blocked, 'a supervisor chain that loops back is refused';

	select id into child from public.team
		where company_id = '41000000-0000-0000-0000-000000000000'
			and parent_team_id = '41000000-0000-0000-0000-0000000000a1';
	begin
		perform public.team_update('41000000-0000-0000-0000-0000000000a1', jsonb_build_object('parentID', child));
	exception when raise_exception then
		team_loop_blocked := true;
	end;
	assert team_loop_blocked, 'an organization that contains itself is refused';

	begin
		perform public.person_set(
			'41000000-0000-0000-0000-000000000013', '{}'::jsonb, '{"employmentStatus": "retired"}'::jsonb
		);
	exception when raise_exception then
		bad_status_blocked := true;
	end;
	assert bad_status_blocked, 'an employment status the record does not know is refused';
end $$;$block$, 'a loop in the organization is refused');

select lives_ok($block$do $$
declare
	child uuid;
	parent_delete_blocked boolean := false;
	left_behind integer;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);

	select id into child from public.team
		where company_id = '41000000-0000-0000-0000-000000000000'
			and parent_team_id = '41000000-0000-0000-0000-0000000000a1';

	perform public.person_set(
		'41000000-0000-0000-0000-000000000013', '{}'::jsonb, jsonb_build_object('groupID', child)
	);

	begin
		perform public.team_delete('41000000-0000-0000-0000-0000000000a1');
	exception when raise_exception then
		parent_delete_blocked := true;
	end;
	assert parent_delete_blocked, 'an organization with organizations under it cannot be removed';

	left_behind := public.team_delete(child);
	assert left_behind = 1, 'a removal says how many people it left with no organization';
	assert (
		select team_id is null from public.member where id = '41000000-0000-0000-0000-000000000013'
	), 'somebody in a removed organization keeps their directory entry with no organization';
	assert (
		select count(*) from public.team where company_id = '41000000-0000-0000-0000-000000000000'
	) = 1, 'the organizations the removal did not name stay where they were';
end $$;$block$, 'a removed organization leaves its people behind');

select lives_ok($block$do $$
declare
	admin_of_the_company boolean;
begin
	set local role authenticated;
	perform set_config('request.jwt.claim.sub', '41000000-0000-0000-0000-000000000001', true);

	perform public.person_set(
		'41000000-0000-0000-0000-000000000013',
		'{"name": "최견본", "isAdmin": true}'::jsonb,
		'{"employmentStatus": "departed"}'::jsonb
	);

	select is_admin into admin_of_the_company from public.member
		where id = '41000000-0000-0000-0000-000000000013';
	assert admin_of_the_company, 'an administrator raises a colleague to administer the company';
	assert (
		select status from public.member where id = '41000000-0000-0000-0000-000000000013'
	) = 'departed', 'an administrator marks a colleague departed';
end $$;$block$, 'an administrator writes both homes in one call');

select * from finish();
rollback;
