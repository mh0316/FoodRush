package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/foodrush/observability"
	pb "github.com/jesus-acev/user-service/pb"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *pb.CreateUserRequest) (string, error) {
	const query = `
		INSERT INTO users (nombre, correo, payment_token, password)
		VALUES ($1, $2, $3, $4)
		RETURNING id;
	`

	dbCtx, span := observability.StartDBSpan(ctx, "postgres", query)
	defer span.End()

	var id string
	err := r.db.QueryRowContext(dbCtx, query, user.Nombre, user.Correo, user.PaymentToken, user.Password).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*pb.User, error) {
	const query = `
		SELECT id, nombre, correo, payment_token
		FROM users
		WHERE id = $1;
	`

	dbCtx, span := observability.StartDBSpan(ctx, "postgres", query)
	defer span.End()

	user := &pb.User{}
	err := r.db.QueryRowContext(dbCtx, query, id).Scan(&user.Id, &user.Nombre, &user.Correo, &user.PaymentToken)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func IsAlreadyExists(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate key value violates unique constraint")
}
