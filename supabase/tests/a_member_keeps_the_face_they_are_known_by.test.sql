begin;
create extension if not exists pgtap with schema extensions;
select plan(9);

insert into auth.users (id, email) values
  ('45000000-0000-0000-0000-000000000001', 'drawn-a@example.test'),
  ('45000000-0000-0000-0000-000000000002', 'drawn-b@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('45000000-0000-0000-0000-0000000000a0', '샘플회사', 'drawn-sample', 'KR', 'ko', 'Asia/Seoul'),
  ('45000000-0000-0000-0000-0000000000a1', '예시회사', 'drawn-example', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, name, profile_image) values
  ('45000000-0000-0000-0000-0000000000b0', '45000000-0000-0000-0000-0000000000a0', 'drawn-a@example.test', '45000000-0000-0000-0000-000000000001', '이샘플', null),
  ('45000000-0000-0000-0000-0000000000b1', '45000000-0000-0000-0000-0000000000a0', 'drawn-b@example.test', '45000000-0000-0000-0000-000000000002', '박예시', '45000000-0000-0000-0000-0000000000a0/shared/face/chosen.png');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"45000000-0000-0000-0000-000000000001"}', true);
  assert public.member_picture_keep('45000000-0000-0000-0000-0000000000a0/shared/member/drawn.png'),
    'a member with no picture was not given the drawn one';
  assert not public.member_picture_keep('45000000-0000-0000-0000-0000000000a0/shared/member/drawn.png'),
    'the same picture was written again';
  reset role;
end $$;$block$, 'a member with no picture keeps one, and the same one is not written twice');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"45000000-0000-0000-0000-000000000001"}', true);
  assert public.member_picture_keep('45000000-0000-0000-0000-0000000000a0/shared/member/messenger.jpg'),
    'the messenger picture did not replace the one the app kept';
  reset role;
end $$;$block$, 'a picture the app kept is replaced by the one the member is known by');

select is(
  (select profile_image from public.member where id = '45000000-0000-0000-0000-0000000000b0'),
  '45000000-0000-0000-0000-0000000000a0/shared/member/messenger.jpg',
  'the member now shows the picture they are known by'
);

select isnt_empty($$
  select 1 from public.abandoned_asset where path = '45000000-0000-0000-0000-0000000000a0/shared/member/drawn.png'
$$, 'the picture it replaced is left to be collected');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"45000000-0000-0000-0000-000000000002"}', true);
  assert not public.member_picture_keep('45000000-0000-0000-0000-0000000000a0/shared/member/drawn-b.png'),
    'a kept picture replaced one chosen elsewhere';
  reset role;
end $$;$block$, 'a picture chosen outside the member folder is left alone');

select is(
  (select profile_image from public.member where id = '45000000-0000-0000-0000-0000000000b1'),
  '45000000-0000-0000-0000-0000000000a0/shared/face/chosen.png',
  'the chosen picture is still there'
);

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"45000000-0000-0000-0000-000000000001"}', true);
  perform public.member_picture_keep('45000000-0000-0000-0000-0000000000a1/shared/member/other.png');
end $$;$block$, '22023', null, 'a picture kept under another company is refused');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"45000000-0000-0000-0000-000000000001"}', true);
  perform public.member_picture_keep('45000000-0000-0000-0000-0000000000a0/shared/company/logo.png');
end $$;$block$, '22023', null, 'a picture kept outside the member folder is refused');

select throws_ok($block$do $$
begin
  set local role anon;
  perform set_config('request.jwt.claims', '{}', true);
  perform public.member_picture_keep('45000000-0000-0000-0000-0000000000a0/shared/member/drawn.png');
end $$;$block$, '42501', null, 'a signed-out caller cannot keep a picture');

select * from finish();
rollback;
