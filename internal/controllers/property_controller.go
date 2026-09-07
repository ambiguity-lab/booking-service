package controllers

import (
	"net/http"

	"github.com/ambiguity-lab/booking-service/internal/repositories"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// propertyController serves property list and detail requests.
type propertyController struct {
	repo *repositories.PropertyRepository
}

func (c *propertyController) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, offset, err := parsePagination(r)
	if err != nil {
		writeError(w, err)
		return
	}
	params := repositories.PropertySearchParams{
		Query:  q.Get("q"),
		City:   q.Get("city"),
		Sort:   q.Get("sort"),
		Limit:  limit,
		Offset: offset,
	}
	props, err := c.repo.Search(r.Context(), params)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, props)
}

func (c *propertyController) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, fieldError("id", "must be a valid UUID"))
		return
	}
	prop, err := c.repo.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, prop)
}
