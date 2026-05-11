package repository

import (
	"database/sql"

	"github.com/gonzalo-fch/PaymentsService/internal/models"
)

type PaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(payment *models.Payment) error {
	return r.db.QueryRow(`
INSERT INTO payments (id, order_id, user_id, amount, metodo_pago_token, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at;
`, payment.ID, payment.OrderID, payment.UserID, payment.Amount, payment.MetodoPagoToken, payment.Status).Scan(&payment.ID, &payment.CreatedAt)
}

func (r *PaymentRepository) GetByOrderID(orderID string) (*models.Payment, error) {
	payment := &models.Payment{}
	err := r.db.QueryRow(`
SELECT id, order_id, user_id, amount, metodo_pago_token, status, created_at
FROM payments
WHERE order_id = $1;
`, orderID).Scan(
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
