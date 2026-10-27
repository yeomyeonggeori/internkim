update public.member
set preferences = jsonb_set(member.preferences, '{notificationSettings}', member.notification_settings)
where member.notification_settings <> '{}'::jsonb;

create or replace function public.my_notification_settings()
returns jsonb
language sql
security definer
stable
set search_path = ''
as $$
  select coalesce(member.preferences -> 'notificationSettings', '{}'::jsonb)
  from public.member
  where member.id = public.my_member();
$$;

create or replace function public.notification_settings_set(chosen jsonb)
returns void
language sql
security definer
set search_path = ''
as $$
  update public.member
  set preferences = jsonb_set(member.preferences, '{notificationSettings}', coalesce(chosen, '{}'::jsonb)),
    notification_settings = coalesce(chosen, '{}'::jsonb)
  where member.id = public.my_member();
$$;
