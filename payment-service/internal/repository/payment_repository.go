package repository

import (
	"context"
	"database/sql"

	"github.com/foodrush/observability"
	"github.com/gonzalo-fch/PaymentsService/internal/models"
)

type PaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(ctx context.Context, payment *models.Payment) error {
	const query = `
		INSERT INTO payments (id, order_id, user_id, amount, metodo_pago_token, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at;
	`

	dbCtx, span := observability.StartDBSpan(ctx, "postgres", query)
	defer span.End()

	return r.db.QueryRowContext(dbCtx, query,
		payment.ID, payment.OrderID, payment.UserID, payment.Amount,
		payment.MetodoPagoToken, payment.Status,
	).Scan(&payment.ID, &payment.CreatedAt)
}

func (r *PaymentRepository) GetByOrderID(ctx context.Context, orderID string) (*models.Payment, error) {
	const query = `
		SELECT id, order_id, user_id, amount, metodo_pago_token, status, created_at
		FROM payments
		WHERE order_id = $1;
	`

	dbCtx, span := observability.StartDBSpan(ctx, "postgres", query)
	defer span.End()

	payment := &models.Payment{}
	err := r.db.QueryRowContext(dbCtx, query, orderID).Scan(
		&payment.ID,
		&payment.OrderID,
		&payment.UserID,
		&payment.Amount,
		&payment.MetodoPagoToken,
		&payment.Status,
		&payment.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return payment, nil
}
