package payment

import (
	"context"
	"fmt"

	"github.com/unofi/unofi/internal/database"
)

// Service orchestrates payment operations across different payment methods.
type Service struct {
	handlers map[Method]MethodHandler
	repo     TransactionRepository
}

// NewService creates a new payment service.
func NewService(repo TransactionRepository) *Service {
	return &Service{
		handlers: make(map[Method]MethodHandler),
		repo:     repo,
	}
}

// RegisterHandler adds a payment method handler.
func (s *Service) RegisterHandler(handler MethodHandler) {
	s.handlers[handler.Name()] = handler
}

// Process initiates a payment through the specified method.
func (s *Service) Process(ctx context.Context, req Request) (*Result, error) {
	handler, ok := s.handlers[req.Method]
	if !ok {
		return nil, fmt.Errorf("unsupported payment method: %s", req.Method)
	}

	result, err := handler.Initiate(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("payment initiate: %w", err)
	}

	// Record the transaction
	tx := &Transaction{
		DeviceID:      req.DeviceID,
		PaymentMethod: req.Method,
		Amount:        req.Amount,
		MBGranted:     result.MBGranted,
		TimeGranted:   result.TimeGranted,
		Status:        result.Status,
		Reference:     result.Reference,
	}
	recorded, err := s.repo.Create(tx)
	if err != nil {
		return nil, fmt.Errorf("record transaction: %w", err)
	}
	result.TransactionID = recorded.ID

	return result, nil
}

// Cancel cancels a payment.
func (s *Service) Cancel(ctx context.Context, method Method, transactionID int64) error {
	handler, ok := s.handlers[method]
	if !ok {
		return fmt.Errorf("unsupported payment method: %s", method)
	}
	return handler.Cancel(ctx, transactionID)
}

// GetStatus returns the status of a transaction.
func (s *Service) GetStatus(ctx context.Context, method Method, transactionID int64) (Status, error) {
	handler, ok := s.handlers[method]
	if !ok {
		return "", fmt.Errorf("unsupported payment method: %s", method)
	}
	return handler.Status(ctx, transactionID)
}

// Ensure Service has access to database (used for future expansion)
var _ = database.DB{}
