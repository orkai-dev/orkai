package pg

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func TestProjectListQueriesHaveNoJoinAliases(t *testing.T) {
	db := bun.NewDB(nil, pgdialect.New())
	id := uuid.New()
	var projects []model.Project

	listByOrg := db.NewSelect().Model(&projects).
		Where("org_id = ?", id).
		OrderExpr("created_at DESC")
	listByTeams := db.NewSelect().Model(&projects).
		Where("org_id = ?", id).
		Where("team_id IN (?)", bun.List([]uuid.UUID{id})).
		OrderExpr("created_at DESC")

	for name, q := range map[string]*bun.SelectQuery{
		"ListByOrg":   listByOrg,
		"ListByTeams": listByTeams,
	} {
		sql := q.String()
		t.Log(name, sql)
		if strings.Contains(sql, " JOIN ") {
			t.Fatalf("%s query must not join teams: %s", name, sql)
		}
		where := sql[strings.Index(sql, "WHERE"):]
		for _, forbidden := range []string{`"project"."org_id"`, `"p"."org_id"`, `"project"."created_at"`, `"p"."created_at"`} {
			if strings.Contains(where, forbidden) {
				t.Fatalf("%s WHERE/ORDER must not contain %q: %s", name, forbidden, sql)
			}
		}
	}
}
