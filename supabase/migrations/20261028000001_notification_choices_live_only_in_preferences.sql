create or replace function public.notification_settings_set(chosen jsonb)
returns void
language sql
security definer
set search_path = ''
as $$
  update public.member
  set preferences = jsonb_set(member.preferences, '{notificationSettings}', coalesce(chosen, '{}'::jsonb))
  where member.id = public.my_member();
$$;

do $$
begin
  if exists (
    select 1 from public.member
    where member.notification_settings <> coalesce(member.preferences -> 'notificationSettings', '{}'::jsonb)
  ) then
    raise exception 'a member holds a notification choice that preferences.notificationSettings lacks; copy it there before dropping member.notification_settings';
  end if;
end $$;

alter table public.member drop column notification_settings;
