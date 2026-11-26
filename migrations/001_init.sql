-- =====================================================================
--  Presensi QR Muda-Mudi  |  Skema Supabase PostgreSQL
--  Jalankan seluruh isi file ini di Supabase > SQL Editor > New query
-- =====================================================================

create extension if not exists pgcrypto;

-- ---------- Master: Desa & Kelompok ----------------------------------
create table if not exists villages (
    id          bigserial primary key,
    name        text not null check (length(trim(name)) > 0),
    created_at  timestamptz not null default now()
);
create unique index if not exists villages_name_uq on villages (lower(name));

create table if not exists groups (
    id          bigserial primary key,
    village_id  bigint not null references villages(id) on delete restrict,
    name        text not null check (length(trim(name)) > 0),
    created_at  timestamptz not null default now()
);
create unique index if not exists groups_village_name_uq on groups (village_id, lower(name));
create index if not exists groups_village_idx on groups (village_id);

-- ---------- Akun admin / pengurus ------------------------------------
create table if not exists admins (
    id             bigserial primary key,
    username       text not null check (username = lower(username) and length(username) >= 3),
    full_name      text not null,
    password_hash  text not null,
    role           text not null default 'pengurus' check (role in ('admin','pengurus')),
    is_active      boolean not null default true,
    created_at     timestamptz not null default now()
);
create unique index if not exists admins_username_uq on admins (username);

-- ---------- Anggota muda-mudi ----------------------------------------
create table if not exists members (
    id             uuid primary key default gen_random_uuid(),
    full_name      text not null check (length(trim(full_name)) > 0),
    phone          text not null default '',
    group_id       bigint not null references groups(id) on delete restrict,
    photo_url      text,
    qr_code_token  uuid not null unique default gen_random_uuid(),
    is_active      boolean not null default true,
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now()
);
create index if not exists members_group_idx     on members (group_id);
create index if not exists members_name_idx      on members (lower(full_name));
-- qr_code_token sudah otomatis ber-index lewat constraint UNIQUE

-- ---------- Jadwal pengajian -----------------------------------------
create table if not exists schedules (
    id          bigserial primary key,
    title       text not null check (length(trim(title)) > 0),
    event_date  date not null,
    start_time  time,
    location    text not null default '',
    is_active   boolean not null default true,
    created_by  bigint references admins(id) on delete set null,
    created_at  timestamptz not null default now()
);
create index if not exists schedules_date_idx on schedules (event_date desc);

-- ---------- Presensi -------------------------------------------------
create table if not exists attendances (
    id           bigserial primary key,
    member_id    uuid   not null references members(id)   on delete cascade,
    schedule_id  bigint references schedules(id)          on delete restrict,
    scanned_by   bigint references admins(id)             on delete set null,
    scanned_at   timestamptz not null default now(),
    -- tanggal lokal WIB (UTC+7); backend juga mengirim nilai ini secara eksplisit
    scan_date    date not null default ((now() at time zone 'Asia/Jakarta')::date),
    constraint attendances_member_day_uq unique (member_id, scan_date)
);
create index if not exists attendances_date_idx     on attendances (scan_date);
create index if not exists attendances_schedule_idx on attendances (schedule_id);
create index if not exists attendances_scanned_idx  on attendances (scanned_at desc);

-- ---------- Keamanan -------------------------------------------------
-- Aplikasi Go terhubung sebagai role postgres (bypass RLS). Mengaktifkan RLS tanpa
-- policy memastikan tabel TIDAK bisa diakses lewat anon key / REST API Supabase.
alter table villages    enable row level security;
alter table groups      enable row level security;
alter table admins      enable row level security;
alter table members     enable row level security;
alter table schedules   enable row level security;
alter table attendances enable row level security;

-- ---------- Storage foto (opsional) ----------------------------------
insert into storage.buckets (id, name, public)
values ('member-photos', 'member-photos', true)
on conflict (id) do nothing;
