delete from public.credential
where company_id is not null
  and kind in ('buzz', 'mattermost');
