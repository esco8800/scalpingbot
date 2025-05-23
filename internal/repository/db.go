package repository

import (
	"context"
	"database/sql"
	"errors"
	_ "github.com/mattn/go-sqlite3"
)

// User представляет модель пользователя
type User struct {
	TelegramID     string
	Username       string
	ProfitPercent  float64
	OrderSize      float64
	BaseBuyTimeout int
	APIKey         string
	SecretKey      string
	Symbol         string
	CreatedAt      string
}

// UserRepository определяет интерфейс для работы с пользователями
type UserRepository interface {
	CreateUser(ctx context.Context, user User) error
	GetUserByID(ctx context.Context, telegramID string) (User, error)
	UpdateUser(ctx context.Context, user User) error
	DeleteUser(ctx context.Context, telegramID string) error
	GetAllUsers(ctx context.Context) ([]User, error)
}

// SQLiteUserRepository реализует UserRepository с использованием SQLite
type SQLiteUserRepository struct {
	db *sql.DB
}

// NewSQLiteUserRepository создает новый экземпляр SQLiteUserRepository
func NewSQLiteUserRepository(dbPath string) (*SQLiteUserRepository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	return &SQLiteUserRepository{db: db}, nil
}

// Close закрывает соединение с базой
func (r *SQLiteUserRepository) Close() error {
	return r.db.Close()
}

// CreateUser добавляет нового пользователя
func (r *SQLiteUserRepository) CreateUser(ctx context.Context, user User) error {
	query := `
        INSERT INTO users (
            telegram_id, username, profit_percent, order_size, 
            base_buy_timeout, api_key, secret_key, symbol
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `
	_, err := r.db.ExecContext(ctx, query,
		user.TelegramID, user.Username, user.ProfitPercent, user.OrderSize,
		user.BaseBuyTimeout, user.APIKey, user.SecretKey, user.Symbol,
	)
	if err != nil {
		return err
	}
	return nil
}

// GetUserByID получает пользователя по Telegram ID
func (r *SQLiteUserRepository) GetUserByID(ctx context.Context, telegramID string) (User, error) {
	var user User
	query := `
        SELECT telegram_id, username, profit_percent, order_size, 
               base_buy_timeout, api_key, secret_key, symbol, created_at
        FROM users WHERE telegram_id = ?
    `
	row := r.db.QueryRowContext(ctx, query, telegramID)
	err := row.Scan(
		&user.TelegramID, &user.Username, &user.ProfitPercent, &user.OrderSize,
		&user.BaseBuyTimeout, &user.APIKey, &user.SecretKey, &user.Symbol, &user.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return User{}, errors.New("user not found")
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}

// UpdateUser обновляет данные пользователя
func (r *SQLiteUserRepository) UpdateUser(ctx context.Context, user User) error {
	query := `
        UPDATE users SET 
            username = ?, profit_percent = ?, order_size = ?,
            base_buy_timeout = ?, api_key = ?, secret_key = ?, symbol = ?
        WHERE telegram_id = ?
    `
	result, err := r.db.ExecContext(ctx, query,
		user.Username, user.ProfitPercent, user.OrderSize,
		user.BaseBuyTimeout, user.APIKey, user.SecretKey, user.Symbol,
		user.TelegramID,
	)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("user not found")
	}
	return nil
}

// DeleteUser удаляет пользователя по Telegram ID
func (r *SQLiteUserRepository) DeleteUser(ctx context.Context, telegramID string) error {
	query := "DELETE FROM users WHERE telegram_id = ?"
	result, err := r.db.ExecContext(ctx, query, telegramID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("user not found")
	}
	return nil
}

// GetAllUsers возвращает список всех пользователей
func (r *SQLiteUserRepository) GetAllUsers(ctx context.Context) ([]User, error) {
	query := `
        SELECT telegram_id, username, profit_percent, order_size, 
               base_buy_timeout, api_key, secret_key, symbol, created_at
        FROM users
    `
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(
			&user.TelegramID, &user.Username, &user.ProfitPercent, &user.OrderSize,
			&user.BaseBuyTimeout, &user.APIKey, &user.SecretKey, &user.Symbol, &user.CreatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}
