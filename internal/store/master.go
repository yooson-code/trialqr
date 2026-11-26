package store

import (
	"context"

	"presensi-qr/internal/models"
)

func (s *Store) ListVillages(ctx context.Context) ([]models.Village, error) {
	rows, err := s.db.Query(ctx, `
		select v.id, v.name, count(g.id)
		from villages v left join groups g on g.village_id = v.id
		group by v.id order by v.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Village
	for rows.Next() {
		var v models.Village
		if err := rows.Scan(&v.ID, &v.Name, &v.GroupCount); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) ListGroups(ctx context.Context) ([]models.Group, error) {
	rows, err := s.db.Query(ctx, `
		select g.id, g.village_id, g.name, v.name,
		       (select count(*) from members m where m.group_id = g.id)
		from groups g join villages v on v.id = g.village_id
		order by v.name, g.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Group
	for rows.Next() {
		var g models.Group
		if err := rows.Scan(&g.ID, &g.VillageID, &g.Name, &g.VillageName, &g.MemberCount); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// VillageViews menggabungkan desa dengan kelompoknya (satu kali query per tabel).
func (s *Store) VillageViews(ctx context.Context) ([]models.VillageView, error) {
	vs, err := s.ListVillages(ctx)
	if err != nil {
		return nil, err
	}
	gs, err := s.ListGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]models.VillageView, len(vs))
	idx := make(map[int64]int, len(vs))
	for i, v := range vs {
		out[i] = models.VillageView{Village: v}
		idx[v.ID] = i
	}
	for _, g := range gs {
		if i, ok := idx[g.VillageID]; ok {
			out[i].Groups = append(out[i].Groups, g)
		}
	}
	return out, nil
}

func (s *Store) CreateVillage(ctx context.Context, name string) error {
	_, err := s.db.Exec(ctx, `insert into villages (name) values ($1)`, name)
	return err
}

func (s *Store) UpdateVillage(ctx context.Context, id int64, name string) error {
	_, err := s.db.Exec(ctx, `update villages set name = $2 where id = $1`, id, name)
	return err
}

func (s *Store) DeleteVillage(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `delete from villages where id = $1`, id)
	return err
}

func (s *Store) CreateGroup(ctx context.Context, villageID int64, name string) error {
	_, err := s.db.Exec(ctx, `insert into groups (village_id, name) values ($1,$2)`, villageID, name)
	return err
}

func (s *Store) UpdateGroup(ctx context.Context, id int64, name string) error {
	_, err := s.db.Exec(ctx, `update groups set name = $2 where id = $1`, id, name)
	return err
}

func (s *Store) DeleteGroup(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `delete from groups where id = $1`, id)
	return err
}
