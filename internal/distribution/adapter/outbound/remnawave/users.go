package remnawave

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
	"go.uber.org/zap"
)

const pageSize = 100

type panelUser struct {
	Username string `json:"username"`
}

type page struct {
	Response struct {
		Total int         `json:"total"`
		Users []panelUser `json:"users"`
	} `json:"response"`
}

type Users struct {
	client *platformremnawave.Client
	apiKey string
	log    *zap.Logger
}

func NewUsers(client *platformremnawave.Client, apiKey string, log *zap.Logger) *Users {
	return &Users{
		client: client,
		apiKey: apiKey,
		log:    log,
	}
}

func (u *Users) All(ctx context.Context) ([]int64, error) {
	ids := make(map[int64]struct{})
	skipped := 0

	for start := 0; ; start += pageSize {
		p, err := u.fetch(ctx, start)
		if err != nil {
			return nil, err
		}

		added, skip := collect(p.Response.Users, ids)
		skipped += skip

		if len(p.Response.Users) < pageSize || added == 0 {
			break
		}
	}

	if skipped > 0 {
		u.log.Warn("рассылка: часть пользователей панели без telegram id, пропущены",
			zap.Int("skipped", skipped))
	}

	out := make([]int64, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}

	slices.Sort(out)

	return out, nil
}

func collect(users []panelUser, ids map[int64]struct{}) (added, skipped int) {
	for _, user := range users {
		id, err := strconv.ParseInt(user.Username, 10, 64)
		if err != nil || id <= 0 {
			skipped++

			continue
		}

		if _, ok := ids[id]; ok {
			continue
		}

		ids[id] = struct{}{}

		added++
	}

	return added, skipped
}

func (u *Users) fetch(ctx context.Context, start int) (page, error) {
	path := fmt.Sprintf("/api/users?start=%d&size=%d&%s", start, pageSize, u.apiKey)

	p, err := platformremnawave.Do[page](ctx, u.client, http.MethodGet, path, nil)
	if err != nil {
		return page{}, fmt.Errorf("список пользователей панели: %w", err)
	}

	return p, nil
}
