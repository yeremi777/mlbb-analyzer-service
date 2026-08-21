package api

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/yeremi777/mlbb-analyzer-service/internal/staticdata"
	"github.com/yeremi777/mlbb-analyzer-service/internal/store"
)

// listHeroes godoc
//
//	@Summary		List heroes
//	@Description	Returns heroes from the dataset with search, role, lane, and pagination filters.
//	@Tags			heroes
//	@Produce		json
//	@Param			search	query		string	false	"Case-insensitive search by hero name"
//	@Param			role	query		string	false	"Case-insensitive role filter"	example(tank)
//	@Param			lane	query		string	false	"Case-insensitive lane filter"	example(roam)
//	@Param			page	query		int		false	"1-based page number"			default(1)
//	@Param			size	query		int		false	"Heroes per page (1-100)"		default(10)
//	@Success		200		{object}	HeroListResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/api/heroes [get]
func (s *Server) listHeroes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := intParam(q.Get("page"), 1, 1, math.MaxInt)
	size := intParam(q.Get("size"), 10, 1, 100)

	heroes, err := store.ListHeroes(r.Context(), s.db)
	if err != nil {
		s.internalError(w, "list heroes", err)
		return
	}

	filtered := heroes
	if search := strings.ToLower(strings.TrimSpace(q.Get("search"))); search != "" {
		filtered = filterHeroes(filtered, func(h staticdata.Hero) bool {
			return strings.Contains(strings.ToLower(h.Name), search)
		})
	}
	if role := strings.ToLower(strings.TrimSpace(q.Get("role"))); role != "" {
		filtered = filterHeroes(filtered, func(h staticdata.Hero) bool {
			return containsFold(h.Roles, role)
		})
	}
	if lane := strings.ToLower(strings.TrimSpace(q.Get("lane"))); lane != "" {
		filtered = filterHeroes(filtered, func(h staticdata.Hero) bool {
			return containsFold(h.Lanes, lane)
		})
	}

	total := len(filtered)
	pages := 0
	if total > 0 {
		pages = (total + size - 1) / size
	}
	start := min((page-1)*size, total)
	end := min(start+size, total)

	writeJSON(w, http.StatusOK, HeroListResponse{
		Items: filtered[start:end], Page: page, Size: size, Total: total, Pages: pages,
	})
}

// getHero godoc
//
//	@Summary		Get hero detail
//	@Description	Returns one hero by dataset UID.
//	@Tags			heroes
//	@Produce		json
//	@Param			heroId	path		string	true	"Hero UID"	example(tigreal)
//	@Success		200		{object}	staticdata.Hero
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/heroes/{heroId} [get]
func (s *Server) getHero(w http.ResponseWriter, r *http.Request) {
	hero, err := store.GetHero(r.Context(), s.db, r.PathValue("heroId"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "hero_not_found", "Hero was not found in the dataset.")
		return
	}
	if err != nil {
		s.internalError(w, "get hero", err)
		return
	}
	writeJSON(w, http.StatusOK, hero)
}

// listCounters godoc
//
//	@Summary		List hero counters
//	@Description	Returns authored counter matchups for one target hero, with counter hero details joined in.
//	@Tags			heroes
//	@Produce		json
//	@Param			heroId	path		string	true	"Target hero UID"	example(tigreal)
//	@Success		200		{array}		CounterMatchup
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/heroes/{heroId}/counters [get]
func (s *Server) listCounters(w http.ResponseWriter, r *http.Request) {
	ms, ok := s.matchupsForHero(w, r, store.CountersForTarget, "counter_data_not_found",
		"Counter data was not found for the target hero.")
	if !ok {
		return
	}
	out := make([]CounterMatchup, len(ms))
	for i, m := range ms {
		out[i] = CounterMatchup{
			TargetHeroID: m.First, CounterHero: m.Second,
			Reasons: m.Reasons, CounterTypes: m.Types, Proof: m.Proof,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// listSynergies godoc
//
//	@Summary		List hero synergies
//	@Description	Returns authored synergy pairings for one anchor hero, with partner hero details joined in.
//	@Tags			heroes
//	@Produce		json
//	@Param			heroId	path		string	true	"Anchor hero UID"	example(tigreal)
//	@Success		200		{array}		SynergyMatchup
//	@Failure		404		{object}	ErrorResponse
//	@Router			/api/heroes/{heroId}/synergies [get]
func (s *Server) listSynergies(w http.ResponseWriter, r *http.Request) {
	ms, ok := s.matchupsForHero(w, r, store.SynergiesForAnchor, "synergy_data_not_found",
		"Synergy data was not found for the anchor hero.")
	if !ok {
		return
	}
	out := make([]SynergyMatchup, len(ms))
	for i, m := range ms {
		out[i] = SynergyMatchup{
			AnchorHeroID: m.First, SynergyHero: m.Second,
			Reasons: m.Reasons, SynergyTypes: m.Types, Proof: m.Proof,
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) matchupsForHero(
	w http.ResponseWriter, r *http.Request,
	fetch func(context.Context, store.Querier, string) ([]store.HeroMatchup, error),
	emptyCode, emptyMessage string,
) ([]store.HeroMatchup, bool) {
	id := r.PathValue("heroId")
	if _, err := store.GetHero(r.Context(), s.db, id); errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "hero_not_found", "Hero was not found in the dataset.")
		return nil, false
	} else if err != nil {
		s.internalError(w, "get hero", err)
		return nil, false
	}

	ms, err := fetch(r.Context(), s.db, id)
	if err != nil {
		s.internalError(w, "list matchups", err)
		return nil, false
	}
	if len(ms) == 0 {
		writeError(w, http.StatusNotFound, emptyCode, emptyMessage)
		return nil, false
	}
	return ms, true
}
