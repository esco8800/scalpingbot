package config

import (
	"errors"
	"fmt"
	"scalpingbot/internal/tools"

	"github.com/spf13/viper"
)

// Config - структура конфигурации бота
type Config struct {
	ProfitPercent    float64 `mapstructure:"profit_percent" json:"profit_percent,omitempty"`
	DropPercent      float64 `mapstructure:"drop_percent" json:"drop_percent,omitempty"`
	DelaySeconds     int     `mapstructure:"delay_seconds" json:"delay_seconds,omitempty"`
	OrderSize        float64 `mapstructure:"order_size" json:"order_size,omitempty"`
	Deposit          float64 `mapstructure:"deposit" json:"deposit,omitempty"`
	AvailableDeposit float64 `mapstructure:"-" json:"available_deposit,omitempty"` // Не читаем из конфига, вычисляем
	APIKey           string  `mapstructure:"api_key" json:"api_key,omitempty"`
	SecretKey        string  `mapstructure:"secret_key" json:"secret_key,omitempty"`
	Symbol           string  `mapstructure:"symbol" json:"symbol,omitempty"` // Например, "KASUSDT"
	BaseBuyTimeout   int     `mapstructure:"base_buy_timeout" json:"base_buy_timeout,omitempty"`
	TgToken          string  `mapstructure:"tg_token" json:"token,omitempty"`
	TgChatID         int64   `mapstructure:"tg_chat_id"  json:"chat_id,omitempty"`
}

// LoadConfig - загрузка конфигурации через Viper
func LoadConfig() (Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// Значения по умолчанию
	viper.SetDefault("profit_percent", 0.3)
	viper.SetDefault("drop_percent", 1.0)
	viper.SetDefault("delay_seconds", 30)
	viper.SetDefault("order_size", 40.0)
	viper.SetDefault("deposit", 400.0)
	viper.SetDefault("api_key", "")
	viper.SetDefault("secret_key", "")
	viper.SetDefault("symbol", "KASUSDT") // Kaspa как пример

	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			tools.LogErrorf("конфигурационный файл не найден")
			return Config{}, errors.New("конфигурационный файл не найден")
		} else {
			return Config{}, fmt.Errorf("ошибка чтения конфигурации: %v", err)
		}
	}

	viper.AutomaticEnv()

	var cfg Config
	err = viper.Unmarshal(&cfg)
	if err != nil {
		tools.LogErrorf("ошибка разбора конфигурации: %v", err)
		return Config{}, fmt.Errorf("ошибка разбора конфигурации: %v", err)
	}

	// Проверяем обязательные поля
	if cfg.APIKey == "" || cfg.SecretKey == "" {
		return Config{}, fmt.Errorf("API Key и Secret Key обязательны для MEXC")
	}

	cfg.AvailableDeposit = cfg.Deposit
	return cfg, nil
}
