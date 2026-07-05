package repository

import (
	"context"
	"database/sql"

	"github.com/gonzalo-fch/PaymentsService/internal/models"
)

type PaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

type OutboxEvent struct {
	ID        string
	EventType string
	Payload   string
	Headers   string // JSON string
	Processed bool
	CreatedAt string
}

func (r *PaymentRepository) CreateWithOutbox(ctx context.Context, payment *models.Payment, outbox *OutboxEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
INSERT INTO payments (id, order_id, user_id, amount, metodo_pago_token, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING created_at;
`, payment.ID, payment.OrderID, payment.UserID, payment.Amount, payment.MetodoPagoToken, payment.Status).Scan(&payment.CreatedAt)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO outbox (id, event_type, payload, headers)
VALUES ($1, $2, $3, $4);
`, outbox.ID, outbox.EventType, outbox.Payload, outbox.Headers)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PaymentRepository) GetUnprocessedEvents(ctx context.Context) ([]*OutboxEvent, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, event_type, payload, headers, created_at
FROM outbox
WHERE processed = FALSE
ORDER BY created_at ASC
LIMIT 100;
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*OutboxEvent
	for rows.Next() {
		e := &OutboxEvent{}
		if err := rows.Scan(&e.ID, &e.EventType, &e.Payload, &e.Headers, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *PaymentRepository) MarkEventAsProcessed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE outbox SET processed = TRUE WHERE id = $1", id)
	return err
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
