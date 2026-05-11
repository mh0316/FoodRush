package repository

import (
	"context"
	"database/sql"
	"errors"

	pb "github.com/mh0316/catalog/pb"
)

var ErrNotFound = errors.New("not found")

type CatalogRepository struct {
	db *sql.DB
}

func NewCatalogRepository(db *sql.DB) *CatalogRepository {
	return &CatalogRepository{db: db}
}

func (r *CatalogRepository) ListComercios(ctx context.Context, soloActivos bool) ([]*pb.Comercio, error) {
	query := "SELECT id, nombre, direccion, activo FROM comercios"
	args := []any{}
	if soloActivos {
		query += " WHERE activo = true"
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	comercios := make([]*pb.Comercio, 0)
	for rows.Next() {
		c := &pb.Comercio{}
		if err := rows.Scan(&c.Id, &c.Nombre, &c.Direccion, &c.Activo); err != nil {
			return nil, err
		}
		comercios = append(comercios, c)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return comercios, nil
}

func (r *CatalogRepository) GetMenuByComercio(ctx context.Context, comercioID string) ([]*pb.Product, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, nombre, precio, comercio_id, disponible FROM productos WHERE comercio_id = $1", comercioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	productos := make([]*pb.Product, 0)
	for rows.Next() {
		p := &pb.Product{}
		if err := rows.Scan(&p.Id, &p.Nombre, &p.Precio, &p.ComercioId, &p.Disponible); err != nil {
			return nil, err
		}
		productos = append(productos, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return productos, nil
}

func (r *CatalogRepository) GetProductDetails(ctx context.Context, id string) (*pb.Product, error) {
	p := &pb.Product{}
	err := r.db.QueryRowContext(ctx, "SELECT id, nombre, precio, comercio_id, disponible FROM productos WHERE id = $1", id).
		Scan(&p.Id, &p.Nombre, &p.Precio, &p.ComercioId, &p.Disponible)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}
