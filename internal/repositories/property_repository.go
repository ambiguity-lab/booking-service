package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ambiguity-lab/booking-service/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PropertySearchParams holds the filters and pagination options for a property search.
type PropertySearchParams struct {
	// Query is a free-text search expression matched against the property name and description.
	Query string
	// City filters properties to those in the specified city.
	City string
	// Sort controls result ordering. Supported values: "price", "rating", "newest".
	Sort string
	// Limit caps the number of rows returned.
	Limit int
	// Offset skips the specified number of rows before returning results.
	Offset int
}

// PropertyRepository provides data access for properties backed by a PostgreSQL pool.
type PropertyRepository struct {
	pool *pgxpool.Pool
}

// NewPropertyRepository creates a PropertyRepository backed by the given connection pool.
func NewPropertyRepository(pool *pgxpool.Pool) *PropertyRepository {
	return &PropertyRepository{pool: pool}
}

// GetByID returns the property with the given id, or models.ErrNotFound if it does not exist.
func (r *PropertyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Property, error) {
	q := `SELECT id, name, city, country, description, base_price_cents, rating,
	       total_rooms, created_at
	       FROM properties WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	p, err := scanProperty(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get property by id: %w", err)
	}
	return p, nil
}

// Search returns properties matching the given filters, ordered and paginated per params.
// TODO: empty q with sort=relevance is not handled
func (r *PropertyRepository) Search(ctx context.Context, params PropertySearchParams) ([]models.Property, error) {
	var conds []string
	var args []any

	if params.Query != "" {
		args = append(args, params.Query)
		conds = append(conds, fmt.Sprintf("search_vector @@ plainto_tsquery('english', $%d)", len(args)))
	}
	if params.City != "" {
		args = append(args, params.City)
		conds = append(conds, fmt.Sprintf("city ILIKE $%d", len(args)))
	}

	orderBy := "created_at DESC"
	switch params.Sort {
	case "price":
		orderBy = "base_price_cents ASC"
	case "rating":
		orderBy = "rating DESC"
	case "newest", "relevance":
		orderBy = "created_at DESC"
	default:
		orderBy = "created_at DESC"
	}

	rankSel := ", NULL::float8"
	if params.Query != "" {
		rankSel = ", ts_rank(search_vector, plainto_tsquery('english', $1)) as rank"
		orderBy = "rank DESC, " + orderBy
	}

	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	q := fmt.Sprintf(`SELECT id, name, city, country, description, base_price_cents, rating,
	       total_rooms, created_at%s
	       FROM properties%s
	       ORDER BY %s`, rankSel, where, orderBy)

	args = append(args, params.Limit, params.Offset)
	q += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("search properties: %w", err)
	}
	defer rows.Close()

	props := make([]models.Property, 0)
	for rows.Next() {
		var p models.Property
		if err := rows.Scan(&p.ID, &p.Name, &p.City, &p.Country, &p.Description,
			&p.BasePriceCents, &p.Rating, &p.TotalRooms, &p.CreatedAt,
			&p.Rank,
		); err != nil {
			return nil, fmt.Errorf("scan property: %w", err)
		}
		props = append(props, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate properties: %w", err)
	}
	return props, nil
}

// propertyScanner is implemented by both pgx.Row and pgx.Rows to scan a single property.
type propertyScanner interface {
	Scan(dest ...any) error
}

// scanProperty scans one row into a Property.
func scanProperty(s propertyScanner) (*models.Property, error) {
	var p models.Property
	err := s.Scan(&p.ID, &p.Name, &p.City, &p.Country, &p.Description,
		&p.BasePriceCents, &p.Rating, &p.TotalRooms, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
