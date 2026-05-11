package repository

import (
	"database/sql"
	"errors"
	"strings"

	pb "github.com/jesus-acev/user-service/pb"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *pb.CreateUserRequest) (string, error) {
	const query = `
		INSERT INTO users (nombre, correo, payment_token, password)
		VALUES ($1, $2, $3, $4)
		RETURNING id;
	`

	var id string
	err := r.db.QueryRow(query, user.Nombre, user.Correo, user.PaymentToken, user.Password).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *UserRepository) GetByID(id string) (*pb.User, error) {
	const query = `
		SELECT id, nombre, correo, payment_token
		FROM users
		WHERE id = $1;
	`

	user := &pb.User{}
	err := r.db.QueryRow(query, id).Scan(&user.Id, &user.Nombre, &user.Correo, &user.PaymentToken)
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
